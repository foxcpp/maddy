package maddy

import (
	"context"
	"sync"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/module"
)

type testCheck struct {
	connErr   error
	senderErr error
	rcptErr   error
	bodyErr   error
}

func (tc *testCheck) NewMessage(ctx *module.DeliveryContext) (module.CheckState, error) {
	return &testCheckState{ctx, tc}, nil
}

type testCheckState struct {
	ctx   *module.DeliveryContext
	check *testCheck
}

func (tcs *testCheckState) CheckConnection(ctx context.Context) error {
	return tcs.check.connErr
}

func (tcs *testCheckState) CheckSender(ctx context.Context, from string) error {
	return tcs.check.senderErr
}

func (tcs *testCheckState) CheckRcpt(ctx context.Context, to string) error {
	return tcs.check.rcptErr
}

func (tcs *testCheckState) CheckBody(ctx context.Context, headerLock *sync.RWMutex, header textproto.Header, body module.BodyBuffer) error {
	return tcs.check.bodyErr
}

func (tcs *testCheckState) Close() error {
	return nil
}
