package maddy

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap/backend"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
	"github.com/foxcpp/go-sqlmail"
	imapsqlmail "github.com/foxcpp/go-sqlmail/imap"
)

type SQLMail struct {
	*imapsqlmail.Backend
	instName string
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

	cfg := config.Map{}
	cfg.String("driver", false, true, "", &driver)
	cfg.String("dsn", false, true, "", &dsn)
	cfg.Int64("appendlimit", false, false, 32*1024*1024, &appendlimitVal)

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

	mod := SQLMail{
		Backend:  sqlm,
		instName: instName,
	}

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
			return errors.New("Deliver: missing domain part")
		}

		if _, ok := ctx.Opts["local_only"]; ok {
			hostname := ctx.Opts["hostname"]
			if hostname == "" {
				hostname = ctx.OurHostname
			}

			if parts[1] != hostname {
				continue
			}
		}
		if _, ok := ctx.Opts["remote_only"]; ok {
			hostname := ctx.Opts["hostname"]
			if hostname == "" {
				hostname = ctx.OurHostname
			}

			if parts[1] == hostname {
				continue
			}
		}

		u, err := sqlm.GetExistingUser(parts[0])
		if err != nil {
			return err
		}

		// TODO: We need to handle Ctx["spam"] here.
		tgtMbox := "INBOX"

		mbox, err := u.GetMailbox(tgtMbox)
		if err != nil {
			if err == backend.ErrNoSuchMailbox {
				// Create INBOX if it doesn't exists.
				if err := u.CreateMailbox(tgtMbox); err != nil {
					return err
				}
				mbox, err = u.GetMailbox(tgtMbox)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if err := mbox.CreateMessage([]string{}, time.Now(), &buf); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	module.Register("sqlmail", NewSQLMail)
}
