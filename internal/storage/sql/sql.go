// Package sql implements SQL-based storage module
// using go-imap-sql library (github.com/foxcpp/go-imap-sql).
//
// Interfaces implemented:
// - module.StorageBackend
// - module.AuthProvider
// - module.DeliveryTarget
package sql

import (
	"errors"
	"fmt"
	"net"
	"os"
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
	"golang.org/x/text/secure/precis"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Storage struct {
	Back     *imapsql.Backend
	instName string
	Log      log.Logger

	junkMbox string

	inlineDriver string
	inlineDsn    []string

	resolver dns.Resolver
}

type delivery struct {
	store    *Storage
	msgMeta  *module.MsgMetadata
	d        imapsql.Delivery
	mailFrom string

	addedRcpts map[string]struct{}
}

func (d *delivery) AddRcpt(rcptTo string) error {
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
		return err
	}

	d.addedRcpts[accountName] = struct{}{}
	return nil
}

func (d *delivery) Body(header textproto.Header, body buffer.Buffer) error {
	if d.msgMeta.Quarantine {
		if err := d.d.SpecialMailbox(specialuse.Junk, d.store.junkMbox); err != nil {
			return err
		}
	}

	header = header.Copy()
	header.Add("Return-Path", "<"+target.SanitizeForHeader(d.mailFrom)+">")
	return d.d.BodyParsed(header, body.Len(), body)
}

func (d *delivery) Abort() error {
	return d.d.Abort()
}

func (d *delivery) Commit() error {
	return d.d.Commit()
}

func (store *Storage) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
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
		resolver: net.DefaultResolver,
	}
	if len(inlineArgs) != 0 {
		if len(inlineArgs) == 1 {
			return nil, errors.New("sql: expected at least 2 arguments")
		}

		store.inlineDriver = inlineArgs[0]
		store.inlineDsn = inlineArgs[1:]
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
	cfg.String("driver", false, false, store.inlineDriver, &driver)
	cfg.StringList("dsn", false, false, store.inlineDsn, &dsn)
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
	cfg.Int("sqlite3_busy_timeout", false, false, 0, &opts.BusyTimeout)
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

	return nil
}

func (store *Storage) IMAPExtensions() []string {
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN", "SPECIAL-USE"}
}

func (store *Storage) CreateMessageLimit() *uint32 {
	return store.Back.CreateMessageLimit()
}

func (store *Storage) Updates() <-chan backend.Update {
	return store.Back.Updates()
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

func (store *Storage) CheckPlain(username, password string) bool {
	accountName, err := prepareUsername(username)
	if err != nil {
		return false
	}

	password, err = precis.OpaqueString.CompareKey(password)
	if err != nil {
		return false
	}

	return store.Back.CheckPlain(accountName, password)
}

func (store *Storage) GetOrCreateUser(username string) (backend.User, error) {
	accountName, err := prepareUsername(username)
	if err != nil {
		return nil, backend.ErrInvalidCredentials
	}

	return store.Back.GetOrCreateUser(accountName)
}

func (store *Storage) Close() error {
	return store.Back.Close()
}

func init() {
	module.Register("sql", New)
}
