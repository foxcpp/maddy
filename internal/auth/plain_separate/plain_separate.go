package plain_separate

import (
	"errors"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

type Auth struct {
	modName  string
	instName string

	userTbls []module.Table
	passwd   []module.PlainAuth

	onlyFirstID bool

	Log log.Logger
}

func NewAuth(modName, instName string, _, inlinargs []string) (module.Module, error) {
	a := &Auth{
		modName:     modName,
		instName:    instName,
		onlyFirstID: false,
		Log:         log.Logger{Name: modName},
	}

	if len(inlinargs) != 0 {
		return nil, errors.New("plain_separate: inline arguments are not used")
	}

	return a, nil
}

func (a *Auth) Name() string {
	return a.modName
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) Init(cfg *config.Map) error {
	cfg.Bool("debug", false, false, &a.Log.Debug)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	return nil
}

func (a *Auth) Lookup(username string) (string, bool, error) {
	ok := len(a.userTbls) == 0
	for _, tbl := range a.userTbls {
		_, tblOk, err := tbl.Lookup(username)
		if err != nil {
			return "", false, fmt.Errorf("plain_separate: underlying table error: %w", err)
		}
		if tblOk {
			ok = true
			break
		}
	}
	if !ok {
		return "", false, nil
	}
	return "", true, nil
}

func (a *Auth) AuthPlain(username, password string) error {
	ok := len(a.userTbls) == 0
	for _, tbl := range a.userTbls {
		_, tblOk, err := tbl.Lookup(username)
		if err != nil {
			return err
		}
		if tblOk {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("user not found in tables")
	}

	var lastErr error
	for _, p := range a.passwd {
		if err := p.AuthPlain(username, password); err != nil {
			lastErr = err
			continue
		}

		return nil
	}
	return lastErr
}

func init() {
	module.RegisterDeprecated("plain_separate", "auth.plain_separate", NewAuth)
	module.Register("auth.plain_separate", NewAuth)
}
