package imapsql2

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy-storage/pkg/delivery"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

func userDoesNotExist(actual error) error {
	return &exterrors.SMTPError{
		Code:         501,
		EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
		Message:      "User does not exist",
		TargetName:   modName,
		Err:          actual,
	}
}

type storeDelivery struct {
	store *Storage
	rcpts map[string]string
	meta  *module.MsgMetadata
	d     *delivery.Delivery
}

func (store *Storage) StartDelivery(ctx context.Context, msgMeta *module.MsgMetadata, _ string) (module.Delivery, error) {
	d, err := store.delivery.StartDelivery(ctx, msgMeta.ID)
	if err != nil {
		return nil, err
	}
	return storeDelivery{
		store: store,
		rcpts: map[string]string{},
		meta:  msgMeta,
		d:     d,
	}, nil
}

func (d storeDelivery) AddRcpt(ctx context.Context, rcptTo string, _ smtp.RcptOptions) error {
	accountName, err := d.store.deliveryNormalize(ctx, rcptTo)
	if err != nil {
		return userDoesNotExist(err)
	}

	if _, ok := d.rcpts[rcptTo]; ok {
		return nil
	}

	err = d.d.AddRcpt(ctx, accountName, delivery.RcptOpts{
		// TODO: Consider meaningful recipient encoding and escaping.
		AdditionalPremable: []byte(
			"Return-Path: <" + target.SanitizeForHeader(d.meta.OriginalFrom) + ">\r\n" +
				"Delivered-To: <" + target.SanitizeForHeader(rcptTo) + ">\r\n",
		),
	})
	if err != nil {
		if errors.Is(err, delivery.ErrUnknownRecipient) {
			return userDoesNotExist(err)
		}
		return err
	}

	d.rcpts[rcptTo] = accountName
	return nil
}

func (d storeDelivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	var overrides map[string]delivery.RcptOverride
	if !d.meta.Quarantine && d.store.filters != nil {
		for rcptAddr, accountName := range d.rcpts {
			folder, flags, err := d.store.filters.IMAPFilter(accountName, rcptAddr, d.meta, header, body)
			if err != nil {
				d.store.log.Error("IMAPFilter failed", err, "rcpt", rcptAddr)
				continue
			}

			if folder != "" || len(flags) > 0 {
				if overrides == nil {
					overrides = make(map[string]delivery.RcptOverride)
				}
				overrides[rcptAddr] = delivery.RcptOverride{
					FolderName: folder,
					Flags:      flags,
				}
			}
		}
	}

	var err error
	if d.meta.Quarantine {
		err = d.d.SelectFolders(ctx, delivery.RoleJunk, overrides)
	} else {
		err = d.d.SelectFolders(ctx, delivery.RoleInbox, overrides)
	}
	if err != nil {
		return fmt.Errorf("SelectFolders: %w", err)
	}

	return d.d.PrepareBodyWithParsedHeader(ctx, header, bufferAdapter{b: body})
}

func (d storeDelivery) Abort(ctx context.Context) error {
	return d.d.Close(ctx)
}

func (d storeDelivery) Commit(ctx context.Context) error {
	return d.d.Commit(ctx)
}

type bufferAdapter struct {
	b buffer.Buffer
}

func (b bufferAdapter) Open(ctx context.Context) (io.ReadCloser, error) {
	return b.b.Open()
}

func (b bufferAdapter) ReadBytes(ctx context.Context) ([]byte, error) {
	if mb, ok := b.b.(buffer.MemoryBuffer); ok {
		return mb.Slice, nil
	}

	rc, err := b.b.Open()
	if err != nil {
		return nil, err
	}
	defer func(rc io.ReadCloser) {
		err := rc.Close()
		if err != nil {
			log.Println("error closing buffer", err)
		}
	}(rc)
	return io.ReadAll(rc)
}

func (b bufferAdapter) Len() int {
	return b.b.Len()
}

func (b bufferAdapter) Close() error {
	return b.b.Remove()
}
