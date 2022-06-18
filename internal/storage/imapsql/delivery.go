/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package imapsql

import (
	"context"
	"runtime/trace"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/textproto"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

type addedRcpt struct {
	rcptTo string
}
type delivery struct {
	store    *Storage
	msgMeta  *module.MsgMetadata
	d        imapsql.Delivery
	mailFrom string

	addedRcpts map[string]addedRcpt
}

func (d *delivery) String() string {
	return d.store.Name() + ":" + d.store.InstanceName()
}

func userDoesNotExist(actual error) error {
	return &exterrors.SMTPError{
		Code:         501,
		EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
		Message:      "User does not exist",
		TargetName:   "imapsql",
		Err:          actual,
	}
}

func (d *delivery) AddRcpt(ctx context.Context, rcptTo string) error {
	defer trace.StartRegion(ctx, "sql/AddRcpt").End()

	accountName, err := d.store.deliveryNormalize(ctx, rcptTo)
	if err != nil {
		return userDoesNotExist(err)
	}

	if _, ok := d.addedRcpts[accountName]; ok {
		return nil
	}

	// This header is added to the message only for that recipient.
	// go-imap-sql does certain optimizations to store the message
	// with small amount of per-recipient data in a efficient way.
	userHeader := textproto.Header{}
	userHeader.Add("Delivered-To", accountName)

	if err := d.d.AddRcpt(accountName, userHeader); err != nil {
		if err == imapsql.ErrUserDoesntExists || err == backend.ErrNoSuchMailbox {
			return userDoesNotExist(err)
		}
		if _, ok := err.(imapsql.SerializationError); ok {
			return &exterrors.SMTPError{
				Code:         453,
				EnhancedCode: exterrors.EnhancedCode{4, 3, 2},
				Message:      "Internal server error, try again later",
				TargetName:   "imapsql",
				Err:          err,
			}
		}
		return err
	}

	d.addedRcpts[accountName] = addedRcpt{
		rcptTo: rcptTo,
	}
	return nil
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "sql/Body").End()

	if !d.msgMeta.Quarantine && d.store.filters != nil {
		for rcpt, rcptData := range d.addedRcpts {
			folder, flags, err := d.store.filters.IMAPFilter(rcpt, rcptData.rcptTo, d.msgMeta, header, body)
			if err != nil {
				d.store.Log.Error("IMAPFilter failed", err, "rcpt", rcpt)
				continue
			}
			d.d.UserMailbox(rcpt, folder, flags)
		}
	}

	if d.msgMeta.Quarantine {
		if err := d.d.SpecialMailbox(imap.JunkAttr, d.store.junkMbox); err != nil {
			if _, ok := err.(imapsql.SerializationError); ok {
				return &exterrors.SMTPError{
					Code:         453,
					EnhancedCode: exterrors.EnhancedCode{4, 3, 2},
					Message:      "Storage access serialiation problem, try again later",
					TargetName:   "imapsql",
					Err:          err,
				}
			}
			return err
		}
	}

	header = header.Copy()
	header.Add("Return-Path", "<"+target.SanitizeForHeader(d.mailFrom)+">")
	err := d.d.BodyParsed(header, body.Len(), body)
	if _, ok := err.(imapsql.SerializationError); ok {
		return &exterrors.SMTPError{
			Code:         453,
			EnhancedCode: exterrors.EnhancedCode{4, 3, 2},
			Message:      "Storage access serialiation problem, try again later",
			TargetName:   "imapsql",
			Err:          err,
		}
	}
	return err
}

func (d *delivery) Abort(ctx context.Context) error {
	defer trace.StartRegion(ctx, "sql/Abort").End()

	return d.d.Abort()
}

func (d *delivery) Commit(ctx context.Context) error {
	defer trace.StartRegion(ctx, "sql/Commit").End()

	return d.d.Commit()
}

func (store *Storage) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	defer trace.StartRegion(ctx, "sql/Start").End()

	return &delivery{
		store:      store,
		msgMeta:    msgMeta,
		mailFrom:   mailFrom,
		d:          store.Back.NewDelivery(),
		addedRcpts: map[string]addedRcpt{},
	}, nil
}
