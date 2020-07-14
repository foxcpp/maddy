//+build ignore

/*
This is example of a minimal stateful check module implementation.
See HACKING.md in the repo root for implementation recommendations.
*/

package directory_name_here

import (
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

func (c *Check) CheckStateForMsg(msgMeta *module.MsgMetadata) (module.CheckState, error) {
	return &state{
		c:       c,
		msgMeta: msgMeta,
		log:     target.DeliveryLogger(c.log, msgMeta),
	}, nil
}

func (s *state) CheckConnection() module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckSender(addr string) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckRcpt(addr string) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) CheckBody(hdr textproto.Header, body buffer.Buffer) module.CheckResult {
	return module.CheckResult{}
}

func (s *state) Close() error {
	return nil
}

func init() {
	module.Register(modName, New)
}
