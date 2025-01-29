/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
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

func (ea *ExternalAuth) Configure(inlineArgs []string, cfg *config.Map) error {
	if len(inlineArgs) != 0 {
		return errors.New("external: inline arguments are not used")
	}

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
	module.Register("auth.external", NewExternalAuth)
}
