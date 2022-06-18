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

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Static struct {
	modName  string
	instName string

	m map[string][]string
}

func NewStatic(modName, instName string, _, _ []string) (module.Module, error) {
	return &Static{
		modName:  modName,
		instName: instName,
		m:        map[string][]string{},
	}, nil
}

func (s *Static) Init(cfg *config.Map) error {
	cfg.Callback("entry", func(_ *config.Map, node config.Node) error {
		if len(node.Args) < 2 {
			return config.NodeErr(node, "expected at least one value")
		}
		s.m[node.Args[0]] = node.Args[1:]
		return nil
	})
	_, err := cfg.Process()
	return err
}

func (s *Static) Name() string {
	return s.modName
}

func (s *Static) InstanceName() string {
	return s.modName
}

func (s *Static) Lookup(ctx context.Context, key string) (string, bool, error) {
	val := s.m[key]
	if len(val) == 0 {
		return "", false, nil
	}
	return val[0], true, nil
}

func (s *Static) LookupMulti(ctx context.Context, key string) ([]string, error) {
	return s.m[key], nil
}

func init() {
	module.Register("table.static", NewStatic)
}
