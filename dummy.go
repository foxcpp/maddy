package maddy

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

// Dummy is a struct that implements AuthProvider, DeliveryTarget and Filter
// interfaces but does nothing. Useful for testing.
type Dummy struct{ instName string }

func (d *Dummy) CheckPlain(_, _ string) bool {
	return true
}

func (d *Dummy) Name() string {
	return "dummy"
}

func (d *Dummy) InstanceName() string {
	return d.instName
}

func (d *Dummy) Init(_ *config.Map) error {
	return nil
}

func (d *Dummy) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	return dummyDelivery{}, nil
}

type dummyDelivery struct{}

func (dd dummyDelivery) AddRcpt(to string) error {
	return nil
}

func (dd dummyDelivery) Body(header textproto.Header, body buffer.Buffer) error {
	return nil
}

func (dd dummyDelivery) Abort() error {
	return nil
}

func (dd dummyDelivery) Commit() error {
	return nil
}

func init() {
	module.Register("dummy", nil)
	module.RegisterInstance(&Dummy{instName: "dummy"}, nil)
}
