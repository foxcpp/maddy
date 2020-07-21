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

package check

import (
	"context"
	"fmt"
	"runtime/trace"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

type (
	StatelessCheckContext struct {
		// Embedded context.Context value, used for tracing, cancellation and
		// timeouts.
		context.Context

		// Resolver that should be used by the check for DNS queries.
		Resolver dns.Resolver

		MsgMeta *module.MsgMetadata

		// Logger that should be used by the check for logging, note that it is
		// already wrapped to append Msg ID to all messages so check code
		// should not do the same.
		Logger log.Logger
	}
	FuncConnCheck   func(checkContext StatelessCheckContext) module.CheckResult
	FuncSenderCheck func(checkContext StatelessCheckContext, mailFrom string) module.CheckResult
	FuncRcptCheck   func(checkContext StatelessCheckContext, rcptTo string) module.CheckResult
	FuncBodyCheck   func(checkContext StatelessCheckContext, header textproto.Header, body buffer.Buffer) module.CheckResult
)

type statelessCheck struct {
	modName  string
	instName string
	resolver dns.Resolver
	logger   log.Logger

	// One used by Init if config option is not passed by a user.
	defaultFailAction modconfig.FailAction
	// The actual fail action that should be applied.
	failAction modconfig.FailAction

	connCheck   FuncConnCheck
	senderCheck FuncSenderCheck
	rcptCheck   FuncRcptCheck
	bodyCheck   FuncBodyCheck
}

type statelessCheckState struct {
	c       *statelessCheck
	msgMeta *module.MsgMetadata
}

func (s *statelessCheckState) String() string {
	return s.c.modName + ":" + s.c.instName
}

func (s *statelessCheckState) CheckConnection(ctx context.Context) module.CheckResult {
	if s.c.connCheck == nil {
		return module.CheckResult{}
	}
	defer trace.StartRegion(ctx, s.c.modName+"/CheckConnection").End()

	originalRes := s.c.connCheck(StatelessCheckContext{
		Context:  ctx,
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	})
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckSender(ctx context.Context, mailFrom string) module.CheckResult {
	if s.c.senderCheck == nil {
		return module.CheckResult{}
	}
	defer trace.StartRegion(ctx, s.c.modName+"/CheckSender").End()

	originalRes := s.c.senderCheck(StatelessCheckContext{
		Context:  ctx,
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, mailFrom)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckRcpt(ctx context.Context, rcptTo string) module.CheckResult {
	if s.c.rcptCheck == nil {
		return module.CheckResult{}
	}
	defer trace.StartRegion(ctx, s.c.modName+"/CheckRcpt").End()

	originalRes := s.c.rcptCheck(StatelessCheckContext{
		Context:  ctx,
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, rcptTo)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	if s.c.bodyCheck == nil {
		return module.CheckResult{}
	}
	defer trace.StartRegion(ctx, s.c.modName+"/CheckBody").End()

	originalRes := s.c.bodyCheck(StatelessCheckContext{
		Context:  ctx,
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, header, body)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) Close() error {
	return nil
}

func (c *statelessCheck) CheckStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &statelessCheckState{
		c:       c,
		msgMeta: msgMeta,
	}, nil
}

func (c *statelessCheck) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &c.logger.Debug)
	cfg.Custom("fail_action", false, false,
		func() (interface{}, error) {
			return c.defaultFailAction, nil
		}, modconfig.FailActionDirective, &c.failAction)
	_, err := cfg.Process()
	return err
}

func (c *statelessCheck) Name() string {
	return c.modName
}

func (c *statelessCheck) InstanceName() string {
	return c.instName
}

// RegisterStatelessCheck is helper function to create stateless message check modules
// that run one simple check during one stage.
//
// It creates the module and its instance with the specified name that implement module.Check interface
// and runs passed functions when corresponding module.CheckState methods are called.
//
// Note about CheckResult that is returned by the functions:
// StatelessCheck supports different action types based on the user configuration, but the particular check
// code doesn't need to know about it. It should assume that it is always "Reject" and hence it should
// populate Reason field of the result object with the relevant error description.
func RegisterStatelessCheck(name string, defaultFailAction modconfig.FailAction, connCheck FuncConnCheck, senderCheck FuncSenderCheck, rcptCheck FuncRcptCheck, bodyCheck FuncBodyCheck) {
	module.Register(name, func(modName, instName string, aliases, inlineArgs []string) (module.Module, error) {
		if len(inlineArgs) != 0 {
			return nil, fmt.Errorf("%s: inline arguments are not used", modName)
		}
		return &statelessCheck{
			modName:  modName,
			instName: instName,
			resolver: dns.DefaultResolver(),
			logger:   log.Logger{Name: modName},

			defaultFailAction: defaultFailAction,

			connCheck:   connCheck,
			senderCheck: senderCheck,
			rcptCheck:   rcptCheck,
			bodyCheck:   bodyCheck,
		}, nil
	})
}
