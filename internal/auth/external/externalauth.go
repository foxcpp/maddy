package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

type ExternalAuth struct {
	modName    string
	instName   string
	helperPath string

	perDomain bool
	domains   []string

	Log log.Logger
}

func NewExternalAuth(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	ea := &ExternalAuth{
		modName:  modName,
		instName: instName,
		Log:      log.Logger{Name: modName},
	}

	if len(inlineArgs) != 0 {
		return nil, errors.New("external: inline arguments are not used")
	}

	return ea, nil
}

func (ea *ExternalAuth) Name() string {
	return ea.modName
}

func (ea *ExternalAuth) InstanceName() string {
	return ea.instName
}

func (ea *ExternalAuth) Init(cfg *config.Map) error {
	cfg.Bool("debug", false, false, &ea.Log.Debug)
	cfg.Bool("perdomain", false, false, &ea.perDomain)
	cfg.StringList("domains", false, false, nil, &ea.domains)
	cfg.String("helper", false, false, "", &ea.helperPath)
	if _, err := cfg.Process(); err != nil {
		return err
	}
	if ea.perDomain && ea.domains == nil {
		return errors.New("auth_domains must be set if auth_perdomain is used")
	}

	if ea.helperPath != "" {
		ea.Log.Debugln("using helper:", ea.helperPath)
	} else {
		ea.helperPath = filepath.Join(config.LibexecDirectory, "maddy-auth-helper")
	}
	if _, err := os.Stat(ea.helperPath); err != nil {
		return fmt.Errorf("%s doesn't exist", ea.helperPath)
	}

	ea.Log.Debugln("using helper:", ea.helperPath)

	return nil
}

func (ea *ExternalAuth) AuthPlain(username, password string) error {
	accountName, ok := auth.CheckDomainAuth(username, ea.perDomain, ea.domains)
	if !ok {
		return module.ErrUnknownCredentials
	}

	return AuthUsingHelper(ea.helperPath, accountName, password)
}

func init() {
	module.Register("extauth", NewExternalAuth)
}
