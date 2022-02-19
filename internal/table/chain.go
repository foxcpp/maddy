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
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/module"
)

type Chain struct {
	modName  string
	instName string

	chain    []module.Table
	optional []bool
}

func NewChain(modName, instName string, _, _ []string) (module.Module, error) {
	return &Chain{
		modName:  modName,
		instName: instName,
	}, nil
}

func (s *Chain) Init(cfg *config.Map) error {
	cfg.Callback("step", func(m *config.Map, node config.Node) error {
		var tbl module.Table
		err := modconfig.ModuleFromNode("table", node.Args, node, m.Globals, &tbl)
		if err != nil {
			return err
		}

		s.chain = append(s.chain, tbl)
		s.optional = append(s.optional, false)
		return nil
	})
	cfg.Callback("optional_step", func(m *config.Map, node config.Node) error {
		var tbl module.Table
		err := modconfig.ModuleFromNode("table", node.Args, node, m.Globals, &tbl)
		if err != nil {
			return err
		}

		s.chain = append(s.chain, tbl)
		s.optional = append(s.optional, true)
		return nil
	})

	_, err := cfg.Process()
	return err
}

func (s *Chain) Name() string {
	return s.modName
}

func (s *Chain) InstanceName() string {
	return s.instName
}

func (s *Chain) Lookup(ctx context.Context, key string) (string, bool, error) {
	newVal, err := s.LookupMulti(ctx, key)
	if err != nil {
		return "", false, err
	}
	if len(newVal) == 0 {
		return "", false, nil
	}

	return newVal[0], true, nil
}

func (s *Chain) LookupMulti(ctx context.Context, key string) ([]string, error) {
	result := []string{key}
STEP:
	for i, step := range s.chain {
		newResult := []string{}
		for _, key = range result {
			if step_multi, ok := step.(module.MultiTable); ok {
				val, err := step_multi.LookupMulti(ctx, key)
				if err != nil {
					return []string{}, err
				}
				if len(val) == 0 {
					if s.optional[i] {
						continue STEP
					}
					return []string{}, nil
				}
				newResult = append(newResult, val...)
			} else {
				val, ok, err := step.Lookup(ctx, key)
				if err != nil {
					return []string{}, err
				}
				if !ok {
					if s.optional[i] {
						continue STEP
					}
					return []string{}, nil
				}
				newResult = append(newResult, val)
			}
		}
		result = newResult
	}
	return result, nil
}

func init() {
	module.Register("table.chain", NewChain)
}
