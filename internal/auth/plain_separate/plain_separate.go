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

package plain_separate

import (
	"context"
	"errors"
	"fmt"

	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
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
	cfg.Callback("user", func(m *config.Map, node config.Node) error {
		var tbl module.Table
		err := modconfig.ModuleFromNode("table", node.Args, node, m.Globals, &tbl)
		if err != nil {
			return err
		}

		a.userTbls = append(a.userTbls, tbl)
		return nil
	})
	cfg.Callback("pass", func(m *config.Map, node config.Node) error {
		var auth module.PlainAuth
		err := modconfig.ModuleFromNode("auth", node.Args, node, m.Globals, &auth)
		if err != nil {
			return err
		}

		a.passwd = append(a.passwd, auth)
		return nil
	})

	if _, err := cfg.Process(); err != nil {
		return err
	}

	return nil
}

func (a *Auth) Lookup(ctx context.Context, username string) (string, bool, error) {
	ok := len(a.userTbls) == 0
	for _, tbl := range a.userTbls {
		_, tblOk, err := tbl.Lookup(ctx, username)
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
		_, tblOk, err := tbl.Lookup(context.TODO(), username)
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
	module.Register("auth.plain_separate", NewAuth)
}
