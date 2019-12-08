package check

import (
	"fmt"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
)

type (
	StatelessCheckContext struct {
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
	defaultFailAction FailAction
	// The actual fail action that should be applied.
	failAction FailAction

	connCheck   FuncConnCheck
	senderCheck FuncSenderCheck
	rcptCheck   FuncRcptCheck
	bodyCheck   FuncBodyCheck
}

type statelessCheckState struct {
	c       *statelessCheck
	msgMeta *module.MsgMetadata
}

func (s *statelessCheckState) CheckConnection() module.CheckResult {
	if s.c.connCheck == nil {
		return module.CheckResult{}
	}

	originalRes := s.c.connCheck(StatelessCheckContext{
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	})
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckSender(mailFrom string) module.CheckResult {
	if s.c.senderCheck == nil {
		return module.CheckResult{}
	}

	originalRes := s.c.senderCheck(StatelessCheckContext{
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, mailFrom)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckRcpt(rcptTo string) module.CheckResult {
	if s.c.rcptCheck == nil {
		return module.CheckResult{}
	}

	originalRes := s.c.rcptCheck(StatelessCheckContext{
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, rcptTo)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) CheckBody(header textproto.Header, body buffer.Buffer) module.CheckResult {
	if s.c.bodyCheck == nil {
		return module.CheckResult{}
	}

	originalRes := s.c.bodyCheck(StatelessCheckContext{
		Resolver: s.c.resolver,
		MsgMeta:  s.msgMeta,
		Logger:   target.DeliveryLogger(s.c.logger, s.msgMeta),
	}, header, body)
	return s.c.failAction.Apply(originalRes)
}

func (s *statelessCheckState) Close() error {
	return nil
}

func (c *statelessCheck) CheckStateForMsg(ctx *module.MsgMetadata) (module.CheckState, error) {
	return &statelessCheckState{
		c:       c,
		msgMeta: ctx,
	}, nil
}

func (c *statelessCheck) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &c.logger.Debug)
	cfg.Custom("fail_action", false, false,
		func() (interface{}, error) {
			return c.defaultFailAction, nil
		}, FailActionDirective, &c.failAction)
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
func RegisterStatelessCheck(name string, defaultFailAction FailAction, connCheck FuncConnCheck, senderCheck FuncSenderCheck, rcptCheck FuncRcptCheck, bodyCheck FuncBodyCheck) {
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
