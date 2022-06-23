//go:build ignore
// +build ignore

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

/*
This is example of a minimal stateful check module implementation.
See HACKING.md in the repo root for implementation recommendations.
*/

package directory_name_here

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

const modName = "check_things"

type Check struct {
	instName string
	log      log.Logger
}

func New(modName, instName string, aliases, inlineArgs []string) (module.Module, error) {
	return &Check{
		instName: instName,
	}, nil
}

func (c *Check) Name() string {
	return modName
}

func (c *Check) InstanceName() string {
	return c.instName
}

func (c *Check) Init(cfg *config.Map) error {
	return nil
}

type state struct {
	c       *Check
	msgMeta *module.MsgMetadata
	log     log.Logger
}

func (c *Check) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func (s *state) CheckConnection(ctx context.Context) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckSender(ctx context.Context, addr string) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckRcpt(ctx context.Context, addr string) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckBody(ctx context.Context, hdr textproto.Header, body buffer.Buffer) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) Close() error {
	return nil
}

func init() {
	module.Register(modName, New)
}
