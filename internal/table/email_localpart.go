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

package table

import (
	"context"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type EmailLocalpart struct {
	modName       string
	instName      string
	allowNonEmail bool
}

func NewEmailLocalpart(modName, instName string) (module.Module, error) {
	return &EmailLocalpart{
		modName:       modName,
		instName:      instName,
		allowNonEmail: modName == "table.email_localpart_optional",
	}, nil
}

func (s *EmailLocalpart) Configure(inlineArgs []string, cfg *config.Map) error {
	return nil
}

func (s *EmailLocalpart) Name() string {
	return s.modName
}

func (s *EmailLocalpart) InstanceName() string {
	return s.modName
}

func (s *EmailLocalpart) Lookup(ctx context.Context, key string) (string, bool, error) {
	mbox, _, err := address.Split(key)
	if err != nil {
		if s.allowNonEmail {
			return key, true, nil
		}
		// Invalid email, no local part mapping.
		return "", false, nil
	}
	return mbox, true, nil
}

func init() {
	module.Register("table.email_localpart", NewEmailLocalpart)
	module.Register("table.email_localpart_optional", NewEmailLocalpart)
}
