package module

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
)

// Dummy is a struct that implements AuthProvider and DeliveryTarget
// interfaces but does nothing. Useful for testing.
//
// It is always registered under the 'dummy' name and can be used in both tests
// and the actual server code (but the latter is kinda pointless).
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

func (d *Dummy) Start(msgMeta *MsgMetadata, mailFrom string) (Delivery, error) {
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
	Register("dummy", func(_, instName string, _, _ []string) (Module, error) {
		return &Dummy{instName: instName}, nil
	})
}
