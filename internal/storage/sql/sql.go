// Package sql implements SQL-based storage module
// using go-imap-sql library (github.com/foxcpp/go-imap-sql).
//
// Interfaces implemented:
// - module.StorageBackend
// - module.PlainAuth
// - module.DeliveryTarget
package sql

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/trace"
	"strconv"
	"strings"

	specialuse "github.com/emersion/go-imap-specialuse"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/textproto"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
	"github.com/foxcpp/maddy/internal/updatepipe"
	"golang.org/x/text/secure/precis"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Storage struct {
	Back     *imapsql.Backend
	instName string
	Log      log.Logger

	junkMbox string

	driver string
	dsn    []string

	resolver dns.Resolver

	updates     <-chan backend.Update
	updPipe     updatepipe.P
	updPushStop chan struct{}
}

type delivery struct {
	store    *Storage
	msgMeta  *module.MsgMetadata
	d        imapsql.Delivery
	mailFrom string

	addedRcpts map[string]struct{}
}

func (d *delivery) String() string {
	return d.store.Name() + ":" + d.store.InstanceName()
}

func (d *delivery) AddRcpt(ctx context.Context, rcptTo string) error {
	defer trace.StartRegion(ctx, "sql/AddRcpt").End()

	accountName, err := prepareUsername(rcptTo)
	if err != nil {
		return &exterrors.SMTPError{
			Code:         501,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
			Message:      "User does not exist",
			TargetName:   "sql",
			Err:          err,
		}
	}

	accountName = strings.ToLower(accountName)
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
			return &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
				Message:      "User does not exist",
				TargetName:   "sql",
				Err:          err,
			}
		}
		if _, ok := err.(imapsql.SerializationError); ok {
			return &exterrors.SMTPError{
				Code:         453,
				EnhancedCode: exterrors.EnhancedCode{4, 3, 2},
				Message:      "Storage access serialiation problem, try again later",
				TargetName:   "sql",
				Err:          err,
			}
		}
		return err
	}

	d.addedRcpts[accountName] = struct{}{}
	return nil
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "sql/Body").End()

	if d.msgMeta.Quarantine {
		if err := d.d.SpecialMailbox(specialuse.Junk, d.store.junkMbox); err != nil {
			if _, ok := err.(imapsql.SerializationError); ok {
				return &exterrors.SMTPError{
					Code:         453,
					EnhancedCode: exterrors.EnhancedCode{4, 3, 2},
					Message:      "Storage access serialiation problem, try again later",
					TargetName:   "sql",
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
			TargetName:   "sql",
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
		addedRcpts: map[string]struct{}{},
	}, nil
}

func (store *Storage) Name() string {
	return "sql"
}

func (store *Storage) InstanceName() string {
	return store.instName
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	store := &Storage{
		instName: instName,
		Log:      log.Logger{Name: "sql"},
		resolver: dns.DefaultResolver(),
	}
	if len(inlineArgs) != 0 {
		if len(inlineArgs) == 1 {
			return nil, errors.New("sql: expected at least 2 arguments")
		}

		store.driver = inlineArgs[0]
		store.dsn = inlineArgs[1:]
	}
	return store, nil
}

func (store *Storage) Init(cfg *config.Map) error {
	var (
		driver          string
		dsn             []string
		fsstoreLocation string
		appendlimitVal  = -1
		compression     []string
	)

	opts := imapsql.Opts{
		// Prevent deadlock if nobody is listening for updates (e.g. no IMAP
		// configured).
		LazyUpdatesInit: true,
	}
	cfg.String("driver", false, false, store.driver, &driver)
	cfg.StringList("dsn", false, false, store.dsn, &dsn)
	cfg.Custom("fsstore", false, false, func() (interface{}, error) {
		return "messages", nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 0 or 1 arguments")
		}
		return node.Args[0], nil
	}, &fsstoreLocation)
	cfg.StringList("compression", false, false, []string{"off"}, &compression)
	cfg.DataSize("appendlimit", false, false, 32*1024*1024, &appendlimitVal)
	cfg.Bool("debug", true, false, &store.Log.Debug)
	cfg.Int("sqlite3_cache_size", false, false, 0, &opts.CacheSize)
	cfg.Int("sqlite3_busy_timeout", false, false, 5000, &opts.BusyTimeout)
	cfg.Bool("sqlite3_exclusive_lock", false, false, &opts.ExclusiveLock)
	cfg.String("junk_mailbox", false, false, "Junk", &store.junkMbox)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if dsn == nil {
		return errors.New("sql: dsn is required")
	}
	if driver == "" {
		return errors.New("sql: driver is required")
	}

	opts.Log = &store.Log

	if appendlimitVal == -1 {
		opts.MaxMsgBytes = nil
	} else {
		// int is 32-bit on some platforms, so cut off values we can't actually
		// use.
		if int(uint32(appendlimitVal)) != appendlimitVal {
			return errors.New("sql: appendlimit value is too big")
		}
		opts.MaxMsgBytes = new(uint32)
		*opts.MaxMsgBytes = uint32(appendlimitVal)
	}
	var err error

	dsnStr := strings.Join(dsn, " ")

	if err := os.MkdirAll(fsstoreLocation, os.ModeDir|os.ModePerm); err != nil {
		return err
	}
	extStore := &imapsql.FSStore{Root: fsstoreLocation}

	if len(compression) != 0 {
		switch compression[0] {
		case "zstd", "lz4":
			opts.CompressAlgo = compression[0]
			if len(compression) == 2 {
				opts.CompressAlgoParams = compression[1]
				if _, err := strconv.Atoi(compression[1]); err != nil {
					return errors.New("sql: first argument for lz4 and zstd is compression level")
				}
			}
			if len(compression) > 2 {
				return errors.New("sql: expected at most 2 arguments")
			}
		case "off":
			if len(compression) > 1 {
				return errors.New("sql: expected at most 1 arguments")
			}
		default:
			return errors.New("sql: unknown compression algorithm")
		}
	}

	store.Back, err = imapsql.New(driver, dsnStr, extStore, opts)
	if err != nil {
		return fmt.Errorf("sql: %s", err)
	}

	store.Log.Debugln("go-imap-sql version", imapsql.VersionStr)

	store.driver = driver
	store.dsn = dsn

	return nil
}

func (store *Storage) EnableUpdatePipe(mode updatepipe.BackendMode) error {
	if store.updPipe != nil {
		return nil
	}
	if store.updates != nil {
		panic("sql: EnableUpdatePipe called after Updates")
	}

	upds := store.Back.Updates()

	switch store.driver {
	case "sqlite3":
		dbId := sha1.Sum([]byte(strings.Join(store.dsn, " ")))
		store.updPipe = &updatepipe.UnixSockPipe{
			SockPath: filepath.Join(
				config.RuntimeDirectory,
				fmt.Sprintf("sql-%s.sock", hex.EncodeToString(dbId[:]))),
			Log: log.Logger{Name: "sql/updpipe", Debug: store.Log.Debug},
		}
	default:
		return errors.New("sql: driver does not have an update pipe implementation")
	}

	wrapped := make(chan backend.Update, cap(upds)*2)

	if mode == updatepipe.ModeReplicate {
		if err := store.updPipe.Listen(wrapped); err != nil {
			return err
		}
	}

	if err := store.updPipe.InitPush(); err != nil {
		return err
	}

	store.updPushStop = make(chan struct{})
	go func() {
		defer func() {
			store.updPushStop <- struct{}{}
			close(wrapped)
		}()

		for {
			select {
			case <-store.updPushStop:
				return
			case u := <-upds:
				if u == nil {
					// The channel is closed. We must be stopping now.
					<-store.updPushStop
					return
				}

				if err := store.updPipe.Push(u); err != nil {
					store.Log.Error("IMAP update pipe push failed", err)
				}

				if mode != updatepipe.ModePush {
					wrapped <- u
				}
			}
		}
	}()

	store.updates = wrapped
	return nil
}

func (store *Storage) I18NLevel() int {
	return 1
}

func (store *Storage) IMAPExtensions() []string {
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN", "SPECIAL-USE", "I18NLEVEL=1"}
}

func (store *Storage) CreateMessageLimit() *uint32 {
	return store.Back.CreateMessageLimit()
}

func (store *Storage) Updates() <-chan backend.Update {
	if store.updates != nil {
		return store.updates
	}

	store.updates = store.Back.Updates()
	return store.updates
}

func (store *Storage) EnableChildrenExt() bool {
	return store.Back.EnableChildrenExt()
}

func prepareUsername(username string) (string, error) {
	mbox, domain, err := address.Split(username)
	if err != nil {
		return "", fmt.Errorf("sql: username prepare: %w", err)
	}

	// PRECIS is not included in the regular address.ForLookup since it reduces
	// the range of valid addresses to a subset of actually valid values.
	// PRECIS is a matter of our own local policy, not a general rule for all
	// email addresses.

	// Side note: For used profiles, there is no practical difference between
	// CompareKey and String.
	mbox, err = precis.UsernameCaseMapped.CompareKey(mbox)
	if err != nil {
		return "", fmt.Errorf("sql: username prepare: %w", err)
	}

	domain, err = dns.ForLookup(domain)
	if err != nil {
		return "", fmt.Errorf("sql: username prepare: %w", err)
	}

	return mbox + "@" + domain, nil
}

func (store *Storage) AuthPlain(username, password string) error {
	// TODO: Pass session context there.
	defer trace.StartRegion(context.Background(), "sql/AuthPlain").End()

	accountName, err := prepareUsername(username)
	if err != nil {
		return err
	}

	password, err = precis.OpaqueString.CompareKey(password)
	if err != nil {
		return err
	}

	// TODO: Make go-imap-sql CheckPlain return an actual error.
	if !store.Back.CheckPlain(accountName, password) {
		return module.ErrUnknownCredentials
	}
	return nil
}

func (store *Storage) GetOrCreateUser(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, backend.ErrInvalidCredentials
	}

	return store.Back.GetOrCreateUser(accountName)
}

func (store *Storage) Close() error {
	// Stop backend from generating new updates.
	store.Back.Close()

	// Wait for 'updates replicate' goroutine to actually stop so we will send
	// all updates before shuting down (this is especially important for
	// maddyctl).
	if store.updPipe != nil {
		store.updPushStop <- struct{}{}
		<-store.updPushStop

		store.updPipe.Close()
	}

	return nil
}

func init() {
	module.Register("sql", New)
}
