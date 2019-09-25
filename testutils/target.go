package testutils

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/module"
)

type Msg struct {
	MsgMeta  *module.MsgMetadata
	MailFrom string
	RcptTo   []string
	Body     []byte
	Header   textproto.Header
}

type Target struct {
	Messages []Msg

	StartErr       error
	RcptErr        map[string]error
	BodyErr        error
	PartialBodyErr map[string]error
	AbortErr       error
	CommitErr      error

	InstName string
}

/*
module.Module is implemented with dummy functions for logging done by Dispatcher code.
*/

func (dt Target) Init(*config.Map) error {
	return nil
}

func (dt Target) InstanceName() string {
	if dt.InstName != "" {
		return dt.InstName
	}
	return "test_instance"
}

func (dt Target) Name() string {
	return "test_target"
}

type testTargetDelivery struct {
	msg       Msg
	tgt       *Target
	aborted   bool
	committed bool
}

type testTargetDeliveryPartial struct {
	testTargetDelivery
}

func (dt *Target) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	if dt.PartialBodyErr != nil {
		return &testTargetDeliveryPartial{
			testTargetDelivery: testTargetDelivery{
				tgt: dt,
				msg: Msg{MsgMeta: msgMeta, MailFrom: mailFrom},
			},
		}, dt.StartErr
	}
	return &testTargetDelivery{
		tgt: dt,
		msg: Msg{MsgMeta: msgMeta, MailFrom: mailFrom},
	}, dt.StartErr
}

func (dtd *testTargetDelivery) AddRcpt(to string) error {
	if dtd.tgt.RcptErr != nil {
		if err := dtd.tgt.RcptErr[to]; err != nil {
			return err
		}
	}

	dtd.msg.RcptTo = append(dtd.msg.RcptTo, to)
	return nil
}

func (dtd *testTargetDeliveryPartial) BodyNonAtomic(c module.StatusCollector, header textproto.Header, buf buffer.Buffer) {
	if dtd.tgt.PartialBodyErr != nil {
		for rcpt, err := range dtd.tgt.PartialBodyErr {
			c.SetStatus(rcpt, err)
		}
		return
	}

	dtd.msg.Header = header

	body, err := buf.Open()
	if err != nil {
		for rcpt, err := range dtd.tgt.PartialBodyErr {
			c.SetStatus(rcpt, err)
		}
	}
	defer body.Close()

	dtd.msg.Body, err = ioutil.ReadAll(body)
}

func (dtd *testTargetDelivery) Body(header textproto.Header, buf buffer.Buffer) error {
	if dtd.tgt.PartialBodyErr != nil {
		return errors.New("partial failure occurred, no additional information available")
	}
	if dtd.tgt.BodyErr != nil {
		return dtd.tgt.BodyErr
	}

	dtd.msg.Header = header

	body, err := buf.Open()
	if err != nil {
		return err
	}
	defer body.Close()

	dtd.msg.Body, err = ioutil.ReadAll(body)
	return err
}

func (dtd *testTargetDelivery) Abort() error {
	return dtd.tgt.AbortErr
}

func (dtd *testTargetDelivery) Commit() error {
	if dtd.tgt.CommitErr != nil {
		return dtd.tgt.CommitErr
	}
	dtd.tgt.Messages = append(dtd.tgt.Messages, dtd.msg)
	return nil
}

func DoTestDelivery(t *testing.T, tgt module.DeliveryTarget, from string, to []string) string {
	t.Helper()

	IDRaw := sha1.Sum([]byte(t.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	body := buffer.MemoryBuffer{Slice: []byte("foobar")}
	ctx := module.MsgMetadata{
		DontTraceSender: true,
		ID:              encodedID,
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

func DoTestDeliveryErr(t *testing.T, tgt module.DeliveryTarget, from string, to []string) (string, error) {
	t.Helper()

	IDRaw := sha1.Sum([]byte(t.Name()))
	encodedID := hex.EncodeToString(IDRaw[:])

	body := buffer.MemoryBuffer{Slice: []byte("foobar")}
	ctx := module.MsgMetadata{
		DontTraceSender: true,
		ID:              encodedID,
	}
	delivery, err := tgt.Start(&ctx, from)
	if err != nil {
		return encodedID, err
	}
	for _, rcpt := range to {
		if err := delivery.AddRcpt(rcpt); err != nil {
			delivery.Abort()
			return encodedID, err
		}
	}
	if err := delivery.Body(textproto.Header{}, body); err != nil {
		delivery.Abort()
		return encodedID, err
	}
	if err := delivery.Commit(); err != nil {
		return encodedID, err
	}

	return encodedID, err
}

func CheckTestMessage(t *testing.T, tgt *Target, indx int, sender string, rcpt []string) {
	t.Helper()

	if len(tgt.Messages) <= indx {
		t.Errorf("wrong amount of messages received, want at least %d, got %d", indx+1, len(tgt.Messages))
		return
	}
	msg := tgt.Messages[indx]

	CheckMsg(t, &msg, sender, rcpt)
}

func CheckMsg(t *testing.T, msg *Msg, sender string, rcpt []string) {
	t.Helper()

	idRaw := sha1.Sum([]byte(t.Name()))
	encodedId := hex.EncodeToString(idRaw[:])

	if msg.MsgMeta.ID != encodedId {
		t.Errorf("empty or wrong delivery context for passed message? %+v", msg.MsgMeta)
	}
	if msg.MailFrom != sender {
		t.Errorf("wrong sender, want %s, got %s", sender, msg.MailFrom)
	}

	sort.Strings(rcpt)
	sort.Strings(msg.RcptTo)
	if !reflect.DeepEqual(msg.RcptTo, rcpt) {
		t.Errorf("wrong recipients, want %v, got %v", rcpt, msg.RcptTo)
	}
	if string(msg.Body) != "foobar" {
		t.Errorf("wrong body, want '%s', got '%s'", "foobar", string(msg.Body))
	}
}
