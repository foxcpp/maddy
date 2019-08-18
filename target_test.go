package maddy

import (
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/maddy/buffer"
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
	msg       msg
	tgt       *testTarget
	aborted   bool
	committed bool
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

func (dtd *testTargetDelivery) Body(header textproto.Header, buf buffer.Buffer) error {
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

func doTestDelivery(t *testing.T, tgt module.DeliveryTarget, from string, to []string) string {
	t.Helper()

	IDRaw := sha1.Sum([]byte(t.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	body := buffer.MemoryBuffer{Slice: []byte("foobar")}
	ctx := module.DeliveryContext{
		DontTraceSender: true,
		DeliveryID:      encodedID,
	}
	delivery, err := tgt.Start(&ctx, from)
	if err != nil {
		t.Fatalf("unexpected Start err: %v", err)
	}
	for _, rcpt := range to {
		if err := delivery.AddRcpt(rcpt); err != nil {
			t.Fatalf("unexpected AddRcpt err for %s: %v", rcpt, err)
		}
	}
	if err := delivery.Body(textproto.Header{}, body); err != nil {
		t.Fatalf("unexpected Body err: %v", err)
	}
	if err := delivery.Commit(); err != nil {
		t.Fatalf("unexpected Commit err: %v", err)
	}

	return encodedID
}

func checkTestMessage(t *testing.T, tgt *testTarget, indx int, sender string, rcpt []string) {
	t.Helper()

	if len(tgt.messages) <= indx {
		t.Errorf("wrong amount of messages received, want at least %d, got %d", indx+1, len(tgt.messages))
		return
	}
	msg := tgt.messages[indx]

	checkMsg(t, &msg, sender, rcpt)
}

func checkMsg(t *testing.T, msg *msg, sender string, rcpt []string) {
	t.Helper()

	idRaw := sha1.Sum([]byte(t.Name()))
	encodedId := hex.EncodeToString(idRaw[:])

	if msg.ctx.DeliveryID != encodedId {
		t.Errorf("empty or wrong delivery context for passed message? %+v", msg.ctx)
	}
	if msg.mailFrom != sender {
		t.Errorf("wrong sender, want %s, got %s", sender, msg.mailFrom)
	}

	sort.Strings(rcpt)
	sort.Strings(msg.rcptTo)
	if !reflect.DeepEqual(msg.rcptTo, rcpt) {
		t.Errorf("wrong recipients, want %v, got %v", rcpt, msg.rcptTo)
	}
	if string(msg.body) != "foobar" {
		t.Errorf("wrong body, want '%s', got '%s'", "foobar", string(msg.body))
	}
}
