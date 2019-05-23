package maddy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	imapsql "github.com/foxcpp/go-imap-sql"
	"github.com/foxcpp/go-imap-sql/fsstore"
)

type SQLStorage struct {
	back     *imapsql.Backend
	instName string
	Log      log.Logger

	storagePerDomain bool
	authPerDomain    bool
	authDomains      []string
}

type Literal struct {
	io.Reader
	length int
}

func (l Literal) Len() int {
	return l.length
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

func (sqlm *SQLStorage) Deliver(ctx module.DeliveryContext, msg io.Reader) error {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, msg); err != nil {
		return err
	}
	for _, rcpt := range ctx.To {
		parts := strings.Split(rcpt, "@")
		if len(parts) != 2 {
			sqlm.Log.Println("malformed address:", rcpt)
			return errors.New("Deliver: missing domain part")
		}

		u, err := sqlm.back.GetUser(parts[0])
		if err != nil {
			sqlm.Log.Debugf("failed to get user for %s (delivery ID = %s): %v", rcpt, ctx.DeliveryID, err)
			if err == imapsql.ErrUserDoesntExists {
				return &smtp.SMTPError{
					Code:    550,
					Message: "Local mailbox doesn't exists",
				}
			}
			return err
		}

		// TODO: We need to handle Ctx["spam"] here.
		tgtMbox := "INBOX"

		mbox, err := u.GetMailbox(tgtMbox)
		if err != nil {
			if err == backend.ErrNoSuchMailbox {
				// Create INBOX if it doesn't exists.
				sqlm.Log.Debugln("creating inbox for", rcpt)
				if err := u.CreateMailbox(tgtMbox); err != nil {
					sqlm.Log.Debugln("inbox creation failed for", rcpt)
					return err
				}
				mbox, err = u.GetMailbox(tgtMbox)
				if err != nil {
					return err
				}
			} else {
				sqlm.Log.Printf("failed to get inbox for %s (delivery ID = %s): %v", rcpt, ctx.DeliveryID, err)
				return err
			}
		}

		headerPrefix := fmt.Sprintf("Return-Path: <%s>\r\n", sanitizeString(ctx.From))
		headerPrefix += fmt.Sprintf("Delivered-To: %s\r\n", sanitizeString(rcpt))

		msg := Literal{
			Reader: io.MultiReader(strings.NewReader(headerPrefix), &buf),
			length: len(headerPrefix) + buf.Len(),
		}
		if err := mbox.CreateMessage([]string{}, time.Now(), msg); err != nil {
			sqlm.Log.Printf("failed to save msg for %s (delivery ID = %s): %v", rcpt, ctx.DeliveryID, err)
			return err
		}
	}
	return nil
}

func init() {
	module.Register("sql", NewSQLStorage)
}
