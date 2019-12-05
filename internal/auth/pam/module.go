package pam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/internal/auth/external"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

type Auth struct {
	instName   string
	useHelper  bool
	helperPath string

	Log log.Logger
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("pam: inline arguments are not used")
	}
	return &Auth{
		instName: instName,
		Log:      log.Logger{Name: modName},
	}, nil
}

func (a *Auth) Name() string {
	return "pam"
}

func (a *Auth) InstanceName() string {
	return a.instName
}

func (a *Auth) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &a.Log.Debug)
	cfg.Bool("use_helper", false, false, &a.useHelper)
	if _, err := cfg.Process(); err != nil {
		return err
	}
	if !canCallDirectly && !a.useHelper {
		return errors.New("pam: this build lacks support for direct libpam invocation, use helper binary")
	}

	if a.useHelper {
		a.helperPath = filepath.Join(config.LibexecDirectory, "maddy-pam-helper")
		if _, err := os.Stat(a.helperPath); err != nil {
			return fmt.Errorf("pam: no helper binary (maddy-pam-helper) found in %s", config.LibexecDirectory)
		}
	}

	return nil
}

func (a *Auth) CheckPlain(username, password string) bool {
	if a.useHelper {
		return external.AuthUsingHelper(a.Log, a.helperPath, username, password)
	}
	err := runPAMAuth(username, password)
	if err != nil {
		if err == ErrInvalidCredentials {
			a.Log.Println(err)
		}
		return false
	}
	return true
}

func init() {
	module.Register("pam", New)
}
