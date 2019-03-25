package maddy

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap/backend"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"github.com/foxcpp/go-sqlmail"
	imapsqlmail "github.com/foxcpp/go-sqlmail/imap"
)

type SQLMail struct {
	*imapsqlmail.Backend
	instName string
	Log      log.Logger
}

func (sqlm *SQLMail) Name() string {
	return "sqlmail"
}

func (sqlm *SQLMail) InstanceName() string {
	return sqlm.instName
}

func (sqlm *SQLMail) Version() string {
	return sqlmail.VersionStr
}

func NewSQLMail(instName string, globalCfg map[string]config.Node, rawCfg config.Node) (module.Module, error) {
	var driver string
	var dsn string
	appendlimitVal := int64(-1)

	opts := imapsqlmail.Opts{}
	mod := SQLMail{
		instName: instName,
		Log:      log.Logger{Out: log.StderrLog, Name: "sqlmail"},
	}

	cfg := config.Map{}
	cfg.String("driver", false, true, "", &driver)
	cfg.String("dsn", false, true, "", &dsn)
	cfg.Int64("appendlimit", false, false, 32*1024*1024, &appendlimitVal)
	cfg.Bool("debug", true, &mod.Log.Debug)

	if _, err := cfg.Process(globalCfg, &rawCfg); err != nil {
		return nil, err
	}

	if appendlimitVal == -1 {
		opts.MaxMsgBytes = nil
	} else {
		opts.MaxMsgBytes = new(uint32)
		*opts.MaxMsgBytes = uint32(appendlimitVal)
	}
	sqlm, err := imapsqlmail.NewBackend(driver, dsn, opts)
	mod.Backend = sqlm

	if err != nil {
		return nil, err
	}

	return &mod, nil
}

func (sqlm *SQLMail) Extensions() []string {
	return []string{"APPENDLIMIT", "MOVE", "CHILDREN"}
}

func (sqlm *SQLMail) Deliver(ctx module.DeliveryContext, msg io.Reader) error {
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

		if _, ok := ctx.Opts["local_only"]; ok {
			hostname := ctx.Opts["hostname"]
			if hostname == "" {
				hostname = ctx.OurHostname
			}

			if parts[1] != hostname {
				sqlm.Log.Debugf("local_only, skipping %s", rcpt)
				continue
			}
		}
		if _, ok := ctx.Opts["remote_only"]; ok {
			hostname := ctx.Opts["hostname"]
			if hostname == "" {
				hostname = ctx.OurHostname
			}

			if parts[1] == hostname {
				sqlm.Log.Debugf("remote_only, skipping %s", rcpt)
				continue
			}
		}

		u, err := sqlm.GetExistingUser(parts[0])
		if err != nil {
			sqlm.Log.Debugf("failed to get user for %s: %v", rcpt, err)
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
				sqlm.Log.Debugf("failed to get inbox for %s: %v", rcpt, err)
				return err
			}
		}

		if err := mbox.CreateMessage([]string{}, time.Now(), &buf); err != nil {
			sqlm.Log.Debugf("failed to save msg for %s: %v", rcpt, err)
			return err
		}
	}
	return nil
}

func init() {
	module.Register("sqlmail", NewSQLMail)
}
