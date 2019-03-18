package maddy

import (
	"errors"
	"fmt"
	"strconv"

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

func init() {
	module.Register("sqlmail", NewSQLMail)
}
