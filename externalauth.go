package maddy

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
)

type ExternalAuth struct {
	modName    string
	instName   string
	helperPath string

	Log log.Logger
}

func NewExternalAuth(modName, instName string) (module.Module, error) {
	ea := &ExternalAuth{
		modName:  modName,
		instName: instName,
		Log:      log.Logger{Out: log.StderrLog, Name: modName},
	}

	return ea, nil
}

func (ea *ExternalAuth) Name() string {
	return ea.modName
}

func (ea *ExternalAuth) InstanceName() string {
	return ea.instName
}

func (ea *ExternalAuth) Init(globalCfg map[string]config.Node, rawCfg config.Node) error {
	cfg := config.Map{}
	cfg.Bool("debug", true, &ea.Log.Debug)
	cfg.String("helper", false, false, "", &ea.helperPath)
	if _, err := cfg.Process(globalCfg, &rawCfg); err != nil {
		return err
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

	var err error
	ea.helperPath, err = exec.LookPath(helperName)
	if err != nil {
		return fmt.Errorf("no %s authentication support, %s is not found in $PATH and no custom path is set", ea.modName, helperName)
	}

	ea.Log.Debugln("using helper:", ea.helperPath)

	return nil
}

func (ea *ExternalAuth) CheckPlain(username, password string) bool {
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

	if _, err := io.WriteString(stdin, username+"\n"); err != nil {
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
