package maddy

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

type (
	StatelessCheckContext struct {
		// Resolver that should be used by the check for DNS queries.
		Resolver Resolver

		MsgMeta *module.MsgMetadata

		// CancelCtx is cancelled if check should be
		// aborted (e.g. its result is no longer meaningful).
		CancelCtx context.Context

		// Logger that should be used by the check for logging, note that it is
		// already wrapped to append Msg ID to all messages so check code
		// should not do the same.
		Logger log.Logger
	}
	FuncConnCheck   func(checkContext StatelessCheckContext) error
	FuncSenderCheck func(checkContext StatelessCheckContext, mailFrom string) error
	FuncRcptCheck   func(checkContext StatelessCheckContext, rcptTo string) error
	FuncBodyCheck   func(checkContext StatelessCheckContext, headerLock *sync.RWMutex, header textproto.Header, body buffer.Buffer) error
)

type statelessCheck struct {
	modName  string
	instName string
	resolver Resolver
	logger   log.Logger

	// One used by Init if config option is not passed by a user.
	defaultFailAction checkFailAction
	// The actual fail action that should be applied.
	failAction checkFailAction
	okScore    int

	connCheck   FuncConnCheck
	senderCheck FuncSenderCheck
	rcptCheck   FuncRcptCheck
	bodyCheck   FuncBodyCheck
}

type statelessCheckState struct {
	c       *statelessCheck
	msgMeta *module.MsgMetadata
}

func deliveryLogger(l log.Logger, msgMeta *module.MsgMetadata) log.Logger {
	out := l.Out
	if out == nil {
		out = log.DefaultLogger.Out
	}

	return log.Logger{
		Out: func(t time.Time, debug bool, str string) {
			out(t, debug, str+" (msg ID = "+msgMeta.ID+")")
		},
		Name:  l.Name,
		Debug: l.Debug,
	}
}

func (s statelessCheckState) CheckConnection(ctx context.Context) error {
	if s.c.connCheck == nil {
		return nil
	}

	err := s.c.connCheck(StatelessCheckContext{
		Resolver:  s.c.resolver,
		MsgMeta:   s.msgMeta,
		CancelCtx: ctx,
		Logger:    deliveryLogger(s.c.logger, s.msgMeta),
	})
	if err == nil {
		atomic.AddInt32(&s.msgMeta.ChecksScore, int32(s.c.okScore))
	}
	return s.c.failAction.apply(s.msgMeta, err)
}

func (s statelessCheckState) CheckSender(ctx context.Context, mailFrom string) error {
	if s.c.senderCheck == nil {
		return nil
	}

	err := s.c.senderCheck(StatelessCheckContext{
		Resolver:  s.c.resolver,
		MsgMeta:   s.msgMeta,
		CancelCtx: ctx,
		Logger:    deliveryLogger(s.c.logger, s.msgMeta),
	}, mailFrom)
	if err == nil {
		atomic.AddInt32(&s.msgMeta.ChecksScore, int32(s.c.okScore))
	}
	return s.c.failAction.apply(s.msgMeta, err)
}

func (s statelessCheckState) CheckRcpt(ctx context.Context, rcptTo string) error {
	if s.c.rcptCheck == nil {
		return nil
	}

	err := s.c.rcptCheck(StatelessCheckContext{
		Resolver:  s.c.resolver,
		MsgMeta:   s.msgMeta,
		CancelCtx: ctx,
		Logger:    deliveryLogger(s.c.logger, s.msgMeta),
	}, rcptTo)
	if err == nil {
		atomic.AddInt32(&s.msgMeta.ChecksScore, int32(s.c.okScore))
	}
	return s.c.failAction.apply(s.msgMeta, err)
}

func (s statelessCheckState) CheckBody(ctx context.Context, headerLock *sync.RWMutex, header textproto.Header, body buffer.Buffer) error {
	if s.c.bodyCheck == nil {
		return nil
	}

	err := s.c.bodyCheck(StatelessCheckContext{
		Resolver:  s.c.resolver,
		MsgMeta:   s.msgMeta,
		CancelCtx: ctx,
		Logger:    deliveryLogger(s.c.logger, s.msgMeta),
	}, headerLock, header, body)
	if err == nil {
		atomic.AddInt32(&s.msgMeta.ChecksScore, int32(s.c.okScore))
	}
	return s.c.failAction.apply(s.msgMeta, err)
}

func (s statelessCheckState) Close() error {
	return nil
}

func (c *statelessCheck) NewMessage(ctx *module.MsgMetadata) (module.CheckState, error) {
	return statelessCheckState{
		c:       c,
		msgMeta: ctx,
	}, nil
}

func (c *statelessCheck) Init(m *config.Map) error {
	m.Bool("debug", true, &c.logger.Debug)
	m.Custom("fail_action", false, false,
		func() (interface{}, error) {
			return c.defaultFailAction, nil
		}, checkFailActionDirective, &c.failAction)
	_, err := m.Process()
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
func RegisterStatelessCheck(name string, defaultFailAction checkFailAction, connCheck FuncConnCheck, senderCheck FuncSenderCheck, rcptCheck FuncRcptCheck, bodyCheck FuncBodyCheck) {
	module.Register(name, func(modName, instName string, aliases []string) (module.Module, error) {
		return &statelessCheck{
			modName:  modName,
			instName: instName,
			resolver: net.DefaultResolver,
			logger:   log.Logger{Name: modName},

			defaultFailAction: defaultFailAction,

			connCheck:   connCheck,
			senderCheck: senderCheck,
			rcptCheck:   rcptCheck,
			bodyCheck:   bodyCheck,
		}, nil
	})

	// Here is the problem with global configuration.
	// We can't grab it here because this function is likely
	// called from init(). This RegisterInstance call
	// needs to be moved somewhere after global config parsing
	// so we will be able to pass globals to config.Map constructed
	// here and then let Init access it.
	// TODO.

	module.RegisterInstance(&statelessCheck{
		modName:  name,
		instName: name,
		resolver: net.DefaultResolver,
		logger:   log.Logger{Name: name},

		connCheck:   connCheck,
		senderCheck: senderCheck,
		rcptCheck:   rcptCheck,
		bodyCheck:   bodyCheck,
	}, &config.Map{Block: &config.Node{}})
}
