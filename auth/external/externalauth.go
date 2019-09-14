package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/auth"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

type ExternalAuth struct {
	modName    string
	instName   string
	helperPath string

	perDomain bool
	domains   []string

	Log log.Logger
}

func NewExternalAuth(modName, instName string, _ []string) (module.Module, error) {
	ea := &ExternalAuth{
		modName:  modName,
		instName: instName,
		Log:      log.Logger{Name: modName},
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
	cfg.Bool("auth_perdomain", true, false, &ea.perDomain)
	cfg.StringList("auth_domains", true, false, nil, &ea.domains)
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

func (ea *ExternalAuth) CheckPlain(username, password string) bool {
	accountName, ok := auth.CheckDomainAuth(username, ea.perDomain, ea.domains)
	if !ok {
		return false
	}

	return AuthUsingHelper(ea.Log, ea.helperPath, accountName, password)
}

func init() {
	module.Register("extauth", NewExternalAuth)
}
