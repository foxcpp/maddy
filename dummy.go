package maddy

import (
	"io"

	"github.com/emersion/maddy/module"
)

// Dummy is a struct that implements AuthProvider, DeliveryTarget and Filter
// interfaces but does nothing. Useful for testing.
type Dummy struct{ instName string }

func (d Dummy) CheckPlain(_, _ string) bool {
	return true
}

func (d Dummy) Apply(_ *module.DeliveryContext, _ io.Reader) (io.Reader, error) {
	return nil, nil
}

func (d Dummy) Deliver(_ module.DeliveryContext, _ io.Reader) error {
	return nil
}

func (d Dummy) Name() string {
	return "dummy"
}

func (d Dummy) Version() string {
	return VersionStr
}

func (d Dummy) InstanceName() string {
	return d.instName
}

func init() {
	module.Register("dummy", nil)
	module.RegisterInstance(&Dummy{instName: "dummy"})
}
