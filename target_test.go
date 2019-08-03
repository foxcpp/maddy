package maddy

import (
	"io/ioutil"

	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/module"
)

type msg struct {
	ctx      *module.DeliveryContext
	mailFrom string
	rcptTo   []string
	body     []byte
}

type testTarget struct {
	messages []msg

	startErr  error
	rcptErr   map[string]error
	bodyErr   error
	abortErr  error
	commitErr error

	instName string
}

/*
module.Module is implemented with dummy functions for logging done by Dispatcher code.
*/

func (dt testTarget) Init(*config.Map) error {
	return nil
}

func (dt testTarget) InstanceName() string {
	if dt.instName != "" {
		return dt.instName
	}
	return "test_instance"
}

func (dt testTarget) Name() string {
	return "test_target"
}

type testTargetDelivery struct {
	msg      msg
	tgt      *testTarget
	aborted  bool
	commited bool
}

func (dt *testTarget) Start(ctx *module.DeliveryContext, mailFrom string) (module.Delivery, error) {
	return &testTargetDelivery{
		tgt: dt,
		msg: msg{ctx: ctx.DeepCopy(), mailFrom: mailFrom},
	}, dt.startErr
}

func (dtd *testTargetDelivery) AddRcpt(to string) error {
	if dtd.tgt.rcptErr != nil {
		if err := dtd.tgt.rcptErr[to]; err != nil {
			return err
		}
	}

	dtd.msg.rcptTo = append(dtd.msg.rcptTo, to)
	return nil
}

func (dtd *testTargetDelivery) Body(buf module.BodyBuffer) error {
	if dtd.tgt.bodyErr != nil {
		return dtd.tgt.bodyErr
	}

	body, err := buf.Open()
	if err != nil {
		return err
	}
	defer body.Close()

	dtd.msg.body, err = ioutil.ReadAll(body)
	return err
}

func (dtd *testTargetDelivery) Abort() error {
	return dtd.tgt.abortErr
}

func (dtd *testTargetDelivery) Commit() error {
	if dtd.tgt.commitErr != nil {
		return dtd.tgt.commitErr
	}
	dtd.tgt.messages = append(dtd.tgt.messages, dtd.msg)
	return nil
}
