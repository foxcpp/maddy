package maddy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
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

func NewSQLMail(instName string, cfg config.CfgTreeNode) (module.Module, error) {
	var driver string
	var dsn string
	var appendlimitSet bool

	opts := imapsqlmail.Opts{}
	for _, entry := range cfg.Childrens {
		switch entry.Name {
		case "driver":
			if len(entry.Args) != 1 {
				return nil, fmt.Errorf("driver: expected 1 argument")
			}
			driver = entry.Args[0]
		case "dsn":
			if len(entry.Args) != 1 {
				return nil, fmt.Errorf("dsn: expected 1 argument")
			}
			dsn = entry.Args[0]
		case "appendlimit":
			if len(entry.Args) != 1 {
				return nil, errors.New("appendlimit: expected 1 argument")
			}

			lim, err := strconv.Atoi(entry.Args[0])
			if lim == -1 {
				continue
			}

			lim32 := uint32(lim)
			if err != nil {
				return nil, errors.New("appendlimit: invalid value")
			}
			opts.MaxMsgBytes = &lim32

			appendlimitSet = true
		default:
			return nil, fmt.Errorf("unknown directive: %s", entry.Name)
		}
	}

	if !appendlimitSet {
		lim := uint32(32 * 1024 * 1024) // 32 MiB
		opts.MaxMsgBytes = &lim
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

		if _, ok := ctx.Opts["local-only"]; ok {
			hostname := ctx.Opts["hostname"]
			if hostname == "" {
				hostname = ctx.OurHostname
			}

			if parts[1] != hostname {
				continue
			}
		}
		if _, ok := ctx.Opts["remote-only"]; ok {
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
				if err := u.CreateMailbox(tgtMbox); err != nil {
					return err
				}
				mbox, err = u.GetMailbox(tgtMbox)
				if err != nil {
					return err
				}
			}
			return err
		}

		mbox.CreateMessage([]string{}, time.Now(), &buf)
	}
	return nil
}

func init() {
	module.Register("sqlmail", NewSQLMail)
}