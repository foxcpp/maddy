//go:build !windows
// +build !windows

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

package shadow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth/external"
)

type Auth struct {
	instName   string
	useHelper  bool
	helperPath string

	Log log.Logger
}

func New(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("shadow: inline arguments are not used")
	}
	return &Auth{
		instName: instName,
		Log:      log.Logger{Name: modName},
	}, nil
}

func (a *Auth) Name() string {
	return "shadow"
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

	if a.useHelper {
		a.helperPath = filepath.Join(config.LibexecDirectory, "maddy-shadow-helper")
		if _, err := os.Stat(a.helperPath); err != nil {
			return fmt.Errorf("shadow: no helper binary (maddy-shadow-helper) found in %s", config.LibexecDirectory)
		}
	} else {
		f, err := os.Open("/etc/shadow")
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("shadow: can't read /etc/shadow due to permission error, use helper binary or run maddy as a privileged user")
			}
			return fmt.Errorf("shadow: can't read /etc/shadow: %v", err)
		}
		f.Close()
	}

	return nil
}

func (a *Auth) Lookup(username string) (string, bool, error) {
	if a.useHelper {
		return "", false, fmt.Errorf("shadow: table lookup are not possible when using a helper")
	}

	ent, err := Lookup(username)
	if err != nil {
		if errors.Is(err, ErrNoSuchUser) {
			return "", false, nil
		}
		return "", false, err
	}

	if !ent.IsAccountValid() {
		return "", false, nil
	}

	return "", true, nil
}

func (a *Auth) AuthPlain(username, password string) error {
	if a.useHelper {
		return external.AuthUsingHelper(a.helperPath, username, password)
	}

	ent, err := Lookup(username)
	if err != nil {
		return err
	}

	if !ent.IsAccountValid() {
		return fmt.Errorf("shadow: account is expired")
	}

	if !ent.IsPasswordValid() {
		return fmt.Errorf("shadow: password is expired")
	}

	if err := ent.VerifyPassword(password); err != nil {
		if errors.Is(err, ErrWrongPassword) {
			return module.ErrUnknownCredentials
		}
		return err
	}

	return nil
}

func init() {
	module.Register("auth.shadow", New)
}
