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

package testutils

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Check struct {
	InitErr   error
	EarlyErr  error
	ConnRes   module.CheckResult
	SenderRes module.CheckResult
	RcptRes   module.CheckResult
	BodyRes   module.CheckResult

	ConnCalls   int
	SenderCalls int
	RcptCalls   int
	BodyCalls   int

	UnclosedStates int

	InstName string
}

func (c *Check) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	if c.InitErr != nil {
		return nil, c.InitErr
	}

	c.UnclosedStates++
	return &checkState{msgMeta, c}, nil
}

func (c *Check) Init(*config.Map) error {
	return nil
}

func (c *Check) Name() string {
	return "test_check"
}

func (c *Check) InstanceName() string {
	if c.InstName != "" {
		return c.InstName
	}
	return "test_check"
}

func (c *Check) CheckConnection(ctx context.Context, state *smtp.ConnectionState) error {
	return c.EarlyErr
}

type checkState struct {
	msgMeta *module.MsgMetadata
	check   *Check
}

func (cs *checkState) CheckConnection(ctx context.Context) module.CheckResult {
	cs.check.ConnCalls++
	return cs.check.ConnRes
}

func (cs *checkState) CheckSender(ctx context.Context, from string) module.CheckResult {
	cs.check.SenderCalls++
	return cs.check.SenderRes
}

func (cs *checkState) CheckRcpt(ctx context.Context, to string) module.CheckResult {
	cs.check.RcptCalls++
	return cs.check.RcptRes
}

func (cs *checkState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	cs.check.BodyCalls++
	return cs.check.BodyRes
}

func (cs *checkState) Close() error {
	cs.check.UnclosedStates--
	return nil
}

func init() {
	module.Register("test_check", func(_, _ string, _, _ []string) (module.Module, error) {
		return &Check{}, nil
	})
	module.RegisterInstance(&Check{}, nil)
}
