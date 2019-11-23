package testutils

import (
	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
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
	if c.InstName != "" {
		return c.InstName
	}
	return "test_check"
}

func (c *Check) CheckConnection(*smtp.ConnectionState) error {
	return c.EarlyErr
}

type checkState struct {
	msgMeta *module.MsgMetadata
	check   *Check
}

func (cs *checkState) CheckConnection() module.CheckResult {
	cs.check.ConnCalls++
	return cs.check.ConnRes
}

func (cs *checkState) CheckSender(from string) module.CheckResult {
	cs.check.SenderCalls++
	return cs.check.SenderRes
}

func (cs *checkState) CheckRcpt(to string) module.CheckResult {
	cs.check.RcptCalls++
	return cs.check.RcptRes
}

func (cs *checkState) CheckBody(header textproto.Header, body buffer.Buffer) module.CheckResult {
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
