package testutils

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type Check struct {
	InitErr   error
	ConnRes   module.CheckResult
	SenderRes module.CheckResult
	RcptRes   module.CheckResult
	BodyRes   module.CheckResult

	UnclosedStates int
}

func (c *Check) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
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
	return "test_check"
}

type checkState struct {
	msgMeta *module.MsgMetadata
	check   *Check
}

func (cs *checkState) CheckConnection(ctx context.Context) module.CheckResult {
	return cs.check.ConnRes
}

func (cs *checkState) CheckSender(ctx context.Context, from string) module.CheckResult {
	return cs.check.SenderRes
}

func (cs *checkState) CheckRcpt(ctx context.Context, to string) module.CheckResult {
	return cs.check.RcptRes
}

func (cs *checkState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
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
