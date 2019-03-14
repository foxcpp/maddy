package maddy

import (
	"bytes"

	"github.com/emersion/maddy/module"
)

// Dummy is a struct that implements AuthProvider, DeliveryTarget and Filter
// interfaces but does nothing. Useful for testing.
type Dummy struct{ instName string }

func (d Dummy) CheckPlain(username, password string) bool {
	return true
}

func (d Dummy) Apply(ctx *module.DeliveryContext, msg *bytes.Buffer) error {
	return nil
}

func (d Dummy) Deliver(ctx module.DeliveryContext, msg bytes.Buffer) error {
	return nil
}

func (d Dummy) Name() string {
	return "dummy"
}

func (d Dummy) Version() string {
	return "0.0"
}

func (d Dummy) InstanceName() string {
	return d.instName
}

func init() {
	module.Register("dummy", nil, []string{})
	module.RegisterInstance(&Dummy{instName: "dummy"})
}
