package maddy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func NewExternalAuth(modName, instName string) (module.Module, error) {
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
	cfg.Bool("debug", true, &ea.Log.Debug)
	cfg.Bool("auth_perdomain", true, &ea.perDomain)
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
		return nil
	}

	helperName := "maddy-auth-helper"
	switch ea.modName {
	case "pam":
		helperName = "maddy-pam-helper"
	case "shadow":
		helperName = "maddy-shadow-helper"
	}

	switch ea.modName {
	case "pam", "shadow":
		if ea.perDomain {
			return errors.New("PAM/shadow authentication does not support per-domain namespacing (auth_perdomain)")
		}
	}

	ea.helperPath = filepath.Join(LibexecDirectory(cfg.Globals), helperName)
	if _, err := os.Stat(ea.helperPath); err != nil {
		return fmt.Errorf("no %s authentication support, %s is not found in %s and no custom path is set", ea.modName, LibexecDirectory(cfg.Globals), helperName)
	}

	ea.Log.Debugln("using helper:", ea.helperPath)

	return nil
}

func (ea *ExternalAuth) CheckPlain(username, password string) bool {
	accountName, ok := checkDomainAuth(username, ea.perDomain, ea.domains)
	if !ok {
		return false
	}

	cmd := exec.Command(ea.helperPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		ea.Log.Println("failed to obtain stdin pipe for helper process:", err)
		return false
	}

	if err := cmd.Start(); err != nil {
		ea.Log.Println("failed to start helper process:", err)
		return false
	}

	if _, err := io.WriteString(stdin, accountName+"\n"); err != nil {
		ea.Log.Println("failed to write stdin of helper process:", err)
		return false
	}
	if _, err := io.WriteString(stdin, password+"\n"); err != nil {
		ea.Log.Println("failed to write stdin of helper process:", err)
		return false
	}

	if err := cmd.Wait(); err != nil {
		ea.Log.Debugln(err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 is for authentication failure.
			// Exit code 2 is for other errors.
			if exitErr.ExitCode() == 2 {
				ea.Log.Println(strings.TrimSpace(string(exitErr.Stderr)))
			}
		} else {
			ea.Log.Println("failed to wait for helper process:", err)
		}
		return false
	}

	return true
}

func init() {
	module.Register("extauth", NewExternalAuth)
	module.Register("pam", NewExternalAuth)
	module.Register("shadow", NewExternalAuth)
}
