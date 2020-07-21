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

package msgpipeline

import (
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/module"
)

// CheckGroup is a module container for a group of Check implementations.
//
// It allows to share a set of filter configurations between using named
// configuration blocks (module instances) system.
//
// It is registered globally under the name 'checks'. The object does not
// implement any standard module interfaces besides module.Module and is
// specific to the message pipeline.
type CheckGroup struct {
	instName string
	L        []module.Check
}

func (cg *CheckGroup) Init(cfg *config.Map) error {
	for _, node := range cfg.Block.Children {
		chk, err := modconfig.MessageCheck(cfg.Globals, append([]string{node.Name}, node.Args...), node)
		if err != nil {
			return err
		}

		cg.L = append(cg.L, chk)
	}

	return nil
}

func (CheckGroup) Name() string {
	return "checks"
}

func (cg CheckGroup) InstanceName() string {
	return cg.instName
}

func init() {
	module.Register("checks", func(_, instName string, _, _ []string) (module.Module, error) {
		return &CheckGroup{
			instName: instName,
		}, nil
	})
}
