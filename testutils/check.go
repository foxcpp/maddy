package testutils

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type Check struct {
	ConnRes   module.CheckResult
	SenderRes module.CheckResult
	RcptRes   module.CheckResult
	BodyRes   module.CheckResult
}

func (c *Check) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
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
	return nil
}

func init() {
	module.Register("test_check", func(modName, instanceName string, aliases []string) (module.Module, error) {
		return &Check{}, nil
	})
	module.RegisterInstance(&Check{}, nil)
}
