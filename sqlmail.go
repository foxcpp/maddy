package maddy

import (
	"errors"
	"strconv"

	"github.com/emersion/maddy/module"
	"github.com/foxcpp/go-sqlmail"
	imapsqlmail "github.com/foxcpp/go-sqlmail/imap"
	"github.com/mholt/caddy/caddyfile"
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

func NewSQLMail(instName string, cfg map[string][]caddyfile.Token) (module.Module, error) {
	tokens, ok := cfg["driver"]
	if !ok {
		return nil, errors.New("no driver set")
	}
	driver := oneStringValue(tokens)
	if driver == "" {
		return nil, errors.New("driver: expected 1 argument")
	}

	tokens, ok = cfg["dsn"]
	if !ok {
		return nil, errors.New("no dsn set")
	}
	dsn := oneStringValue(tokens)
	if driver == "" {
		return nil, errors.New("dsn: expected 1 argument")
	}

	opts := imapsqlmail.Opts{}
	if tokens, ok := cfg["appendlimit"]; ok {
		d := caddyfile.NewDispenserTokens("", tokens)
		args := d.RemainingArgs()
		if len(args) != 1 {
			return nil, errors.New("appendlimit: expected 1 argument")
		}

		lim, err := strconv.Atoi(args[0])
		lim32 := uint32(lim)
		if err != nil {
			return nil, errors.New("appendlimit: invalid value")
		}
		opts.MaxMsgBytes = &lim32
	} else {
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
	module.Register("sqlmail", NewSQLMail, []string{"dsn", "driver", "appendlimit"})
}
