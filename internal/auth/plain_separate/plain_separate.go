package plain_separate

import (
	"errors"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"golang.org/x/text/secure/precis"
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

func (a *Auth) AuthPlain(username, password string) ([]string, error) {
	key, err := precis.UsernameCaseMapped.CompareKey(username)
	if err != nil {
		return nil, err
	}

	identities := make([]string, 0, 1)
	if len(a.userTbls) != 0 {
		for _, tbl := range a.userTbls {
			repl, ok, err := tbl.Lookup(key)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if repl != "" {
				identities = append(identities, repl)
			} else {
				identities = append(identities, key)
			}
			if a.onlyFirstID && len(identities) != 0 {
				break
			}
		}
		if len(identities) == 0 {
			return nil, errors.New("plain_separate: unknown credentials")
		}
	}

	var (
		lastErr error
		ok      bool
	)
	for _, pass := range a.passwd {
		passIDs, err := pass.AuthPlain(username, password)
		if err != nil {
			lastErr = err
			continue
		}
		if len(a.userTbls) == 0 {
			identities = append(identities, passIDs...)
		}
		ok = true
	}
	if !ok {
		return nil, lastErr
	}

	return identities, nil
}

func init() {
	module.Register("plain_separate", NewAuth)
}
