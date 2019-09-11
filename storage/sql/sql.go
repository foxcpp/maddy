// Package sql implements SQL-based storage module
// using go-imap-sql library (github.com/foxcpp/go-imap-sql).
//
// Interfaces implemented:
// - module.StorageBackend
// - module.AuthProvider
// - module.DeliveryTarget
package sql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	specialuse "github.com/emersion/go-imap-specialuse"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/go-imap-sql/fsstore"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/auth"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Storage struct {
	back     *imapsql.Backend
	instName string
	Log      log.Logger

	storagePerDomain bool
	authPerDomain    bool
	authDomains      []string
	junkMbox         string

	resolver dns.Resolver
}

type delivery struct {
	store    *Storage
	msgMeta  *module.MsgMetadata
	d        *imapsql.Delivery
	mailFrom string

	addedRcpts map[string]struct{}
}

func LookupAddr(r dns.Resolver, ip net.IP) (string, error) {
	names, err := r.LookupAddr(context.Background(), ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return strings.TrimRight(names[0], "."), nil
}

func sanitizeString(raw string) string {
	return strings.Replace(raw, "\n", "", -1)
}

func generateReceived(r dns.Resolver, msgMeta *module.MsgMetadata, mailFrom, rcptTo string) string {
	var received string
	if !msgMeta.DontTraceSender {
		received += "from " + msgMeta.SrcHostname
		if tcpAddr, ok := msgMeta.SrcAddr.(*net.TCPAddr); ok {
			domain, err := LookupAddr(r, tcpAddr.IP)
			if err != nil {
				received += fmt.Sprintf(" ([%v])", tcpAddr.IP)
			} else {
				received += fmt.Sprintf(" (%s [%v])", domain, tcpAddr.IP)
			}
		}
	}
	received += fmt.Sprintf(" by %s (envelope-sender <%s>)", sanitizeString(msgMeta.OurHostname), sanitizeString(mailFrom))
	received += fmt.Sprintf(" with %s id %s", msgMeta.SrcProto, msgMeta.ID)
	received += fmt.Sprintf(" for %s; %s", rcptTo, time.Now().Format(time.RFC1123Z))
	return received
}

func (d *delivery) AddRcpt(rcptTo string) error {
	var accountName string
	// Side note: <postmaster> address will be always accepted
	// and delivered to "postmaster" account for both cases.
	if d.store.storagePerDomain {
		accountName = rcptTo
	} else {
		var err error
		accountName, _, err = address.Split(rcptTo)
		if err != nil {
			return &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 1, 3},
				Message:      "Invalid recipient address: " + err.Error(),
			}
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
	userHeader.Add("Delivered-To", rcptTo)
	userHeader.Add("Received", generateReceived(d.store.resolver, d.msgMeta, d.mailFrom, rcptTo))

	if err := d.d.AddRcpt(strings.ToLower(accountName), userHeader); err != nil {
		if err == imapsql.ErrUserDoesntExists || err == backend.ErrNoSuchMailbox {
			return &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 1, 1},
				Message:      "User doesn't exist",
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
	header.Add("Return-Path", "<"+sanitizeString(d.mailFrom)+">")
	return d.d.BodyParsed(header, d.msgMeta.BodyLength, body)
}

func (d *delivery) Abort() error {
	return d.d.Abort()
}

func (d *delivery) Commit() error {
	return d.d.Commit()
}

func (store *Storage) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	d, err := store.back.StartDelivery()
	if err != nil {
		return nil, err
	}
	return &delivery{
		store:      store,
		msgMeta:    msgMeta,
		d:          d,
		mailFrom:   mailFrom,
		addedRcpts: map[string]struct{}{},
	}, nil
}

func (store *Storage) Name() string {
	return "sql"
}

func (store *Storage) InstanceName() string {
	return store.instName
}

func New(_, instName string, _ []string) (module.Module, error) {
	return &Storage{
		instName: instName,
		Log:      log.Logger{Name: "sql"},
		resolver: net.DefaultResolver,
	}, nil
}

func (store *Storage) Init(cfg *config.Map) error {
	var driver, dsn string
	var fsstoreLocation string
	appendlimitVal := int64(-1)

	opts := imapsql.Opts{
		// Prevent deadlock if nobody is listening for updates (e.g. no IMAP
		// configured).
		LazyUpdatesInit: true,
	}
	cfg.String("driver", false, true, "", &driver)
	cfg.String("dsn", false, true, "", &dsn)
	cfg.Int64("appendlimit", false, false, 32*1024*1024, &appendlimitVal)
	cfg.Bool("debug", true, false, &store.Log.Debug)
	cfg.Bool("storage_perdomain", true, false, &store.storagePerDomain)
	cfg.Bool("auth_perdomain", true, false, &store.authPerDomain)
	cfg.StringList("auth_domains", true, false, nil, &store.authDomains)
	cfg.Int("sqlite3_cache_size", false, false, 0, &opts.CacheSize)
	cfg.Int("sqlite3_busy_timeout", false, false, 0, &opts.BusyTimeout)
	cfg.Bool("sqlite3_exclusive_lock", false, false, &opts.ExclusiveLock)
	cfg.String("junk_mailbox", false, false, "Junk", &store.junkMbox)

	cfg.Custom("fsstore", false, false, func() (interface{}, error) {
		return "", nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		switch len(node.Args) {
		case 0:
			if store.instName == "" {
				return nil, errors.New("sql: need explicit fsstore location for inline definition")
			}
			return filepath.Join(config.StateDirectory(cfg.Globals), "sql-"+store.instName+"-fsstore"), nil
		case 1:
			return node.Args[0], nil
		default:
			return nil, m.MatchErr("expected 0 or 1 arguments")
		}
	}, &fsstoreLocation)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	opts.Log = &store.Log

	if store.authPerDomain && store.authDomains == nil {
		return errors.New("sql: auth_domains must be set if auth_perdomain is used")
	}

	if fsstoreLocation != "" {
		if !filepath.IsAbs(fsstoreLocation) {
			fsstoreLocation = filepath.Join(config.StateDirectory(cfg.Globals), fsstoreLocation)
		}

		if err := os.MkdirAll(fsstoreLocation, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
		opts.ExternalStore = &fsstore.Store{Root: fsstoreLocation}
	}

	if appendlimitVal == -1 {
		opts.MaxMsgBytes = nil
	} else {
		opts.MaxMsgBytes = new(uint32)
		*opts.MaxMsgBytes = uint32(appendlimitVal)
	}
	var err error
	store.back, err = imapsql.New(driver, dsn, opts)
	if err != nil {
		return fmt.Errorf("sql: %s", err)
	}

	store.Log.Debugln("go-imap-sql version", imapsql.VersionStr)

	return nil
}

func (store *Storage) IMAPExtensions() []string {
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN"}
}

func (store *Storage) Updates() <-chan backend.Update {
	return store.back.Updates()
}

func (store *Storage) EnableChildrenExt() bool {
	return store.back.EnableChildrenExt()
}

func (store *Storage) CheckPlain(username, password string) bool {
	accountName, ok := auth.CheckDomainAuth(username, store.authPerDomain, store.authDomains)
	if !ok {
		return false
	}

	return store.back.CheckPlain(accountName, password)
}

func (store *Storage) GetOrCreateUser(username string) (backend.User, error) {
	var accountName string
	if store.storagePerDomain {
		if !strings.Contains(username, "@") {
			return nil, errors.New("GetOrCreateUser: username@domain required")
		}
		accountName = username
	} else {
		parts := strings.Split(username, "@")
		accountName = parts[0]
	}

	return store.back.GetOrCreateUser(accountName)
}

func init() {
	module.Register("sql", New)
}
