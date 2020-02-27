package module

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
)

// Dummy is a struct that implements PlainAuth and DeliveryTarget
// interfaces but does nothing. Useful for testing.
//
// It is always registered under the 'dummy' name and can be used in both tests
// and the actual server code (but the latter is kinda pointless).
type Dummy struct{ instName string }

func (d *Dummy) AuthPlain(username, _ string) error {
	return nil
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

func (d *Dummy) Start(ctx context.Context, msgMeta *MsgMetadata, mailFrom string) (Delivery, error) {
	return dummyDelivery{}, nil
}

type dummyDelivery struct{}

func (dd dummyDelivery) AddRcpt(ctx context.Context, to string) error {
	return nil
}

func (dd dummyDelivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	return nil
}

func (dd dummyDelivery) Abort(ctx context.Context) error {
	return nil
}

func (dd dummyDelivery) Commit(ctx context.Context) error {
	return nil
}

func init() {
	Register("dummy", func(_, instName string, _, _ []string) (Module, error) {
		return &Dummy{instName: instName}, nil
	})
}
