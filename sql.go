package maddy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/buffer"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/go-imap-sql/fsstore"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type SQLStorage struct {
	back     *imapsql.Backend
	instName string
	Log      log.Logger

	storagePerDomain bool
	authPerDomain    bool
	authDomains      []string
	resolver         Resolver
}

type sqlDelivery struct {
	sqlm     *SQLStorage
	ctx      *module.DeliveryContext
	d        *imapsql.Delivery
	mailFrom string
}

func LookupAddr(r Resolver, ip net.IP) (string, error) {
	names, err := r.LookupAddr(context.Background(), ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return strings.TrimRight(names[0], "."), nil
}

func generateReceived(r Resolver, ctx *module.DeliveryContext, mailFrom, rcptTo string) string {
	var received string
	if !ctx.DontTraceSender {
		received += "from " + ctx.SrcHostname
		if tcpAddr, ok := ctx.SrcAddr.(*net.TCPAddr); ok {
			domain, err := LookupAddr(r, tcpAddr.IP)
			if err != nil {
				received += fmt.Sprintf(" ([%v])", tcpAddr.IP)
			} else {
				received += fmt.Sprintf(" (%s [%v])", domain, tcpAddr.IP)
			}
		}
	}
	received += fmt.Sprintf(" by %s (envelope-sender <%s>)", sanitizeString(ctx.OurHostname), sanitizeString(mailFrom))
	received += fmt.Sprintf(" with %s id %s", ctx.SrcProto, ctx.DeliveryID)
	received += fmt.Sprintf(" for %s; %s", rcptTo, time.Now().Format(time.RFC1123Z))
	return received
}

func (sd *sqlDelivery) AddRcpt(rcptTo string) error {
	var accountName string
	// Side note: <postmaster> address will be always accepted
	// and delivered to "postmaster" account for both cases.
	if sd.sqlm.storagePerDomain {
		accountName = rcptTo
	} else {
		var err error
		accountName, _, err = splitAddress(rcptTo)
		if err != nil {
			return &smtp.SMTPError{
				Code:         501,
				EnhancedCode: smtp.EnhancedCode{5, 1, 3},
				Message:      err.Error(),
			}
		}
	}

	// This header is added to the message only for that recipient.
	// go-imap-sql does certain optimizations to store the message
	// with small amount of per-recipient data in a efficient way.
	userHeader := textproto.Header{}
	userHeader.Add("Delivered-To", rcptTo)
	userHeader.Add("Received", generateReceived(sd.sqlm.resolver, sd.ctx, sd.mailFrom, rcptTo))

	if err := sd.d.AddRcpt(accountName, userHeader); err != nil {
		if err == imapsql.ErrUserDoesntExists || err == backend.ErrNoSuchMailbox {
			return &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 1, 1},
				Message:      "User doesn't exist",
			}
		}
		return err
	}
	return nil
}

func (sd *sqlDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	header = header.Copy()
	header.Add("Return-Path", "<"+sanitizeString(sd.mailFrom)+">")
	return sd.d.BodyParsed(header, sd.ctx.BodyLength, body)
}

func (sd *sqlDelivery) Abort() error {
	return sd.d.Abort()
}

func (sd *sqlDelivery) Commit() error {
	return sd.d.Commit()
}

func (sqlm *SQLStorage) Start(ctx *module.DeliveryContext, mailFrom string) (module.Delivery, error) {
	d, err := sqlm.back.StartDelivery()
	if err != nil {
		return nil, err
	}
	return &sqlDelivery{
		sqlm:     sqlm,
		ctx:      ctx,
		d:        d,
		mailFrom: mailFrom,
	}, nil
}

func (sqlm *SQLStorage) Name() string {
	return "sql"
}

func (sqlm *SQLStorage) InstanceName() string {
	return sqlm.instName
}

func NewSQLStorage(_, instName string) (module.Module, error) {
	return &SQLStorage{
		instName: instName,
		Log:      log.Logger{Name: "sql"},
		resolver: net.DefaultResolver,
	}, nil
}

func (sqlm *SQLStorage) Init(cfg *config.Map) error {
	var driver, dsn string
	var fsstoreLocation string
	appendlimitVal := int64(-1)

	opts := imapsql.Opts{}
	cfg.String("driver", false, true, "", &driver)
	cfg.String("dsn", false, true, "", &dsn)
	cfg.Int64("appendlimit", false, false, 32*1024*1024, &appendlimitVal)
	cfg.Bool("debug", true, &sqlm.Log.Debug)
	cfg.Bool("storage_perdomain", true, &sqlm.storagePerDomain)
	cfg.Bool("auth_perdomain", true, &sqlm.authPerDomain)
	cfg.StringList("auth_domains", true, false, nil, &sqlm.authDomains)
	cfg.Int("sqlite3_cache_size", false, false, 0, &opts.CacheSize)
	cfg.Int("sqlite3_busy_timeout", false, false, 0, &opts.BusyTimeout)
	cfg.Bool("sqlite3_exclusive_lock", false, &opts.ExclusiveLock)

	cfg.Custom("fsstore", false, false, func() (interface{}, error) {
		return "", nil
	}, func(m *config.Map, node *config.Node) (interface{}, error) {
		switch len(node.Args) {
		case 0:
			if sqlm.instName == "" {
				return nil, errors.New("sql: need explicit fsstore location for inline definition")
			}
			return filepath.Join(StateDirectory(cfg.Globals), "sql-"+sqlm.instName+"-fsstore"), nil
		case 1:
			return node.Args[0], nil
		default:
			return nil, m.MatchErr("expected 0 or 1 arguments")
		}
	}, &fsstoreLocation)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if sqlm.authPerDomain && sqlm.authDomains == nil {
		return errors.New("sql: auth_domains must be set if auth_perdomain is used")
	}

	if fsstoreLocation != "" {
		if !filepath.IsAbs(fsstoreLocation) {
			fsstoreLocation = filepath.Join(StateDirectory(cfg.Globals), fsstoreLocation)
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
	sqlm.back, err = imapsql.New(driver, dsn, opts)
	if err != nil {
		return fmt.Errorf("sql: %s", err)
	}

	sqlm.Log.Debugln("go-imap-sql version", imapsql.VersionStr)

	return nil
}

func (sqlm *SQLStorage) IMAPExtensions() []string {
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN"}
}

func (sqlm *SQLStorage) Updates() <-chan backend.Update {
	return sqlm.back.Updates()
}

func (sqlm *SQLStorage) EnableChildrenExt() bool {
	return sqlm.back.EnableChildrenExt()
}

func (sqlm *SQLStorage) CheckPlain(username, password string) bool {
	accountName, ok := checkDomainAuth(username, sqlm.authPerDomain, sqlm.authDomains)
	if !ok {
		return false
	}

	return sqlm.back.CheckPlain(accountName, password)
}

func (sqlm *SQLStorage) GetOrCreateUser(username string) (backend.User, error) {
	var accountName string
	if sqlm.storagePerDomain {
		if !strings.Contains(username, "@") {
			return nil, errors.New("GetOrCreateUser: username@domain required")
		}
		accountName = username
	} else {
		parts := strings.Split(username, "@")
		accountName = parts[0]
	}

	return sqlm.back.GetOrCreateUser(accountName)
}

func init() {
	module.Register("sql", NewSQLStorage)
}
