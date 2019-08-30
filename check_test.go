package maddy

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type testCheck struct {
	connRes   module.CheckResult
	senderRes module.CheckResult
	rcptRes   module.CheckResult
	bodyRes   module.CheckResult
}

func (tc *testCheck) NewMessage(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &testCheckState{msgMeta, tc}, nil
}

func (tc *testCheck) Init(*config.Map) error {
	return nil
}

func (tc *testCheck) Name() string {
	return "test_check"
}

func (tc *testCheck) InstanceName() string {
	return "test_check"
}

type testCheckState struct {
	msgMeta *module.MsgMetadata
	check   *testCheck
}

func (tcs *testCheckState) CheckConnection(ctx context.Context) module.CheckResult {
	return tcs.check.connRes
}

func (tcs *testCheckState) CheckSender(ctx context.Context, from string) module.CheckResult {
	return tcs.check.senderRes
}

func (tcs *testCheckState) CheckRcpt(ctx context.Context, to string) module.CheckResult {
	return tcs.check.rcptRes
}

func (tcs *testCheckState) CheckBody(ctx context.Context, header textproto.Header, body buffer.Buffer) module.CheckResult {
	return tcs.check.bodyRes
}

func (tcs *testCheckState) Close() error {
	return nil
}

func init() {
	module.Register("test_check", func(modName, instanceName string, aliases []string) (module.Module, error) {
		return &testCheck{}, nil
	})
	module.RegisterInstance(&testCheck{}, nil)
}
