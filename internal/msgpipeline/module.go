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
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

type Module struct {
	instName string
	log      log.Logger
	*MsgPipeline
}

func NewModule(modName, instName string, aliases, inlineArgs []string) (module.Module, error) {
	return &Module{
		log:      log.Logger{Name: "msgpipeline"},
		instName: instName,
	}, nil
}

func (m *Module) Init(cfg *config.Map) error {
	var hostname string
	cfg.String("hostname", true, true, "", &hostname)
	cfg.Bool("debug", true, false, &m.log.Debug)
	cfg.AllowUnknown()
	other, err := cfg.Process()
	if err != nil {
		return err
	}

	p, err := New(cfg.Globals, other)
	if err != nil {
		return err
	}
	m.MsgPipeline = p
	m.MsgPipeline.Log = m.log

	return nil
}

func (m *Module) Name() string {
	return "msgpipeline"
}

func (m *Module) InstanceName() string {
	return m.instName
}

func init() {
	module.Register("msgpipeline", NewModule)
}
