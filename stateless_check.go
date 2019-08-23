package maddy

import (
	"context"
	"net"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/buffer"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type (
	StatelessCheckContext struct {
		Resolver    Resolver
		DeliveryCtx *module.DeliveryContext
		CancelCtx   context.Context
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

	connCheck   FuncConnCheck
	senderCheck FuncSenderCheck
	rcptCheck   FuncRcptCheck
	bodyCheck   FuncBodyCheck
}

type statelessCheckState struct {
	c           *statelessCheck
	deliveryCtx *module.DeliveryContext
}

func (s statelessCheckState) CheckConnection(ctx context.Context) error {
	if s.c.connCheck == nil {
		return nil
	}

	return s.c.connCheck(StatelessCheckContext{
		Resolver:    s.c.resolver,
		DeliveryCtx: s.deliveryCtx,
		CancelCtx:   ctx,
	})
}

func (s statelessCheckState) CheckSender(ctx context.Context, mailFrom string) error {
	if s.c.senderCheck == nil {
		return nil
	}

	return s.c.senderCheck(StatelessCheckContext{
		Resolver:    s.c.resolver,
		DeliveryCtx: s.deliveryCtx,
		CancelCtx:   ctx,
	}, mailFrom)
}

func (s statelessCheckState) CheckRcpt(ctx context.Context, rcptTo string) error {
	if s.c.rcptCheck == nil {
		return nil
	}

	return s.c.rcptCheck(StatelessCheckContext{
		Resolver:    s.c.resolver,
		DeliveryCtx: s.deliveryCtx,
		CancelCtx:   ctx,
	}, rcptTo)
}

func (s statelessCheckState) CheckBody(ctx context.Context, headerLock *sync.RWMutex, header textproto.Header, body buffer.Buffer) error {
	if s.c.bodyCheck == nil {
		return nil
	}

	return s.c.bodyCheck(StatelessCheckContext{
		Resolver:    s.c.resolver,
		DeliveryCtx: s.deliveryCtx,
		CancelCtx:   ctx,
	}, headerLock, header, body)
}

func (s statelessCheckState) Close() error {
	return nil
}

func (c *statelessCheck) NewMessage(ctx *module.DeliveryContext) (module.CheckState, error) {
	return statelessCheckState{
		c:           c,
		deliveryCtx: ctx,
	}, nil
}

func (c *statelessCheck) Init(*config.Map) error {
	return nil
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
func RegisterStatelessCheck(name string, connCheck FuncConnCheck, senderCheck FuncSenderCheck, rcptCheck FuncRcptCheck, bodyCheck FuncBodyCheck) {
	module.Register(name, func(modName, instName string) (module.Module, error) {
		return &statelessCheck{
			modName:  modName,
			instName: instName,
			resolver: net.DefaultResolver,

			connCheck:   connCheck,
			senderCheck: senderCheck,
			rcptCheck:   rcptCheck,
			bodyCheck:   bodyCheck,
		}, nil
	})
	module.RegisterInstance(&statelessCheck{
		modName:  name,
		instName: name,
		resolver: net.DefaultResolver,

		connCheck:   connCheck,
		senderCheck: senderCheck,
		rcptCheck:   rcptCheck,
		bodyCheck:   bodyCheck,
	}, nil)
}
