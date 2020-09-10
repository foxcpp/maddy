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

// Package imapsql implements SQL-based storage module
// using go-imap-sql library (github.com/foxcpp/go-imap-sql).
//
// Interfaces implemented:
// - module.StorageBackend
// - module.PlainAuth
// - module.DeliveryTarget
package imapsql

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/trace"
	"strconv"
	"strings"

	"github.com/emersion/go-imap"
	sortthread "github.com/emersion/go-imap-sortthread"
	specialuse "github.com/emersion/go-imap-specialuse"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/textproto"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
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

	filters module.IMAPFilter
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
			TargetName:   "imapsql",
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
				TargetName:   "imapsql",
				Err:          err,
			}
		}
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

	d.addedRcpts[accountName] = struct{}{}
	return nil
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	defer trace.StartRegion(ctx, "sql/Body").End()

	if !d.msgMeta.Quarantine && d.store.filters != nil {
		for rcpt := range d.addedRcpts {
			folder, flags, err := d.store.filters.IMAPFilter(rcpt, d.msgMeta, header, body)
			if err != nil {
				d.store.Log.Error("IMAPFilter failed", err, "rcpt", rcpt)
				continue
			}
			d.d.UserMailbox(rcpt, folder, flags)
		}
	}

	if d.msgMeta.Quarantine {
		if err := d.d.SpecialMailbox(specialuse.Junk, d.store.junkMbox); err != nil {
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
		Log:      log.Logger{Name: "imapsql"},
		resolver: dns.DefaultResolver(),
	}
	if len(inlineArgs) != 0 {
		if len(inlineArgs) == 1 {
			return nil, errors.New("imapsql: expected at least 2 arguments")
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
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, config.NodeErr(node, "expected 0 or 1 arguments")
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
	cfg.Custom("imap_filter", false, false, func() (interface{}, error) {
		return nil, nil
	}, func(m *config.Map, node config.Node) (interface{}, error) {
		var filter module.IMAPFilter
		err := modconfig.GroupFromNode("imap_filters", node.Args, node, m.Globals, &filter)
		return filter, err
	}, &store.filters)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if dsn == nil {
		return errors.New("imapsql: dsn is required")
	}
	if driver == "" {
		return errors.New("imapsql: driver is required")
	}

	opts.Log = &store.Log

	if appendlimitVal == -1 {
		opts.MaxMsgBytes = nil
	} else {
		// int is 32-bit on some platforms, so cut off values we can't actually
		// use.
		if int(uint32(appendlimitVal)) != appendlimitVal {
			return errors.New("imapsql: appendlimit value is too big")
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
					return errors.New("imapsql: first argument for lz4 and zstd is compression level")
				}
			}
			if len(compression) > 2 {
				return errors.New("imapsql: expected at most 2 arguments")
			}
		case "off":
			if len(compression) > 1 {
				return errors.New("imapsql: expected at most 1 arguments")
			}
		default:
			return errors.New("imapsql: unknown compression algorithm")
		}
	}

	store.Back, err = imapsql.New(driver, dsnStr, extStore, opts)
	if err != nil {
		return fmt.Errorf("imapsql: %s", err)
	}

	store.Log.Debugln("go-imap-sql version", imapsql.VersionStr)

	store.driver = driver
	store.dsn = dsn

	store.Back.EnableChildrenExt()
	store.Back.EnableSpecialUseExt()

	return nil
}

func (store *Storage) EnableUpdatePipe(mode updatepipe.BackendMode) error {
	if store.updPipe != nil {
		return nil
	}
	if store.updates != nil {
		panic("imapsql: EnableUpdatePipe called after Updates")
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
		return errors.New("imapsql: driver does not have an update pipe implementation")
	}

	wrapped := make(chan backend.Update, cap(upds)*2)

	if mode == updatepipe.ModeReplicate {
		if err := store.updPipe.Listen(wrapped); err != nil {
			store.updPipe = nil
			return err
		}
	}

	if err := store.updPipe.InitPush(); err != nil {
		store.updPipe = nil
		return err
	}

	store.updPushStop = make(chan struct{})
	go func() {
		defer func() {
			store.updPushStop <- struct{}{}
			close(wrapped)

			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("panic during imapsql update push: %v\n%s", err, stack)
			}
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
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN", "SPECIAL-USE", "I18NLEVEL=1", "SORT", "THREAD=ORDEREDSUBJECT"}
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
		return "", fmt.Errorf("imapsql: username prepare: %w", err)
	}

	// PRECIS is not included in the regular address.ForLookup since it reduces
	// the range of valid addresses to a subset of actually valid values.
	// PRECIS is a matter of our own local policy, not a general rule for all
	// email addresses.

	// Side note: For used profiles, there is no practical difference between
	// CompareKey and String.
	mbox, err = precis.UsernameCaseMapped.CompareKey(mbox)
	if err != nil {
		return "", fmt.Errorf("imapsql: username prepare: %w", err)
	}

	domain, err = dns.ForLookup(domain)
	if err != nil {
		return "", fmt.Errorf("imapsql: username prepare: %w", err)
	}

	return mbox + "@" + domain, nil
}

func (store *Storage) GetOrCreateIMAPAcct(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, backend.ErrInvalidCredentials
	}

	return store.Back.GetOrCreateUser(accountName)
}

func (store *Storage) Lookup(key string) (string, bool, error) {
	accountName, err := prepareUsername(key)
	if err != nil {
		return "", false, nil
	}

	usr, err := store.Back.GetUser(accountName)
	if err != nil {
		if err == imapsql.ErrUserDoesntExists {
			return "", false, nil
		}
		return "", false, err
	}
	if err := usr.Logout(); err != nil {
		store.Log.Error("logout failed", err, "username", accountName)
	}

	return "", true, nil
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

func (store *Storage) Login(_ *imap.ConnInfo, usenrame, password string) (backend.User, error) {
	panic("This method should not be called and is added only to satisfy backend.Backend interface")
}

func (store *Storage) SupportedThreadAlgorithms() []sortthread.ThreadAlgorithm {
	return []sortthread.ThreadAlgorithm{sortthread.OrderedSubject}
}

func init() {
	module.RegisterDeprecated("imapsql", "storage.imapsql", New)
	module.Register("storage.imapsql", New)
	module.Register("target.imapsql", New)
}
