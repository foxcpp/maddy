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
	"fmt"

	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

type EmailWithDomain struct {
	modName  string
	instName string
	domains  []string
	log      log.Logger
}

func NewEmailWithDomain(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &EmailWithDomain{
		modName:  modName,
		instName: instName,
		domains:  inlineArgs,
		log:      log.Logger{Name: modName},
	}, nil
}

func (s *EmailWithDomain) Init(cfg *config.Map) error {
	for _, d := range s.domains {
		if !address.ValidDomain(d) {
			return fmt.Errorf("%s: invalid domain: %s", s.modName, d)
		}
	}
	if len(s.domains) == 0 {
		return fmt.Errorf("%s: at least one domain is required", s.modName)
	}

	return nil
}

func (s *EmailWithDomain) Name() string {
	return s.modName
}

func (s *EmailWithDomain) InstanceName() string {
	return s.modName
}

func (s *EmailWithDomain) Lookup(ctx context.Context, key string) (string, bool, error) {
	quotedMbox := address.QuoteMbox(key)

	if len(s.domains) == 0 {
		s.log.Msg("only first domain is used when expanding key", "key", key, "domain", s.domains[0])
	}

	return quotedMbox + "@" + s.domains[0], true, nil
}

func (s *EmailWithDomain) LookupMulti(ctx context.Context, key string) ([]string, error) {
	quotedMbox := address.QuoteMbox(key)
	emails := make([]string, len(s.domains))
	for i, domain := range s.domains {
		emails[i] = quotedMbox + "@" + domain
	}
	return emails, nil
}

func init() {
	module.Register("table.email_with_domain", NewEmailWithDomain)
}
