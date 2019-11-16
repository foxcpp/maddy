package remote

import (
	"flag"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

// .invalid TLD is used here to make sure if there is something wrong about
// DNS hooks and lookups go to the real Internet, they will not result in
// any useful data that can lead to outgoing connections being made.

func TestRemoteDelivery(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_IPLiteral(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"1.0.0.127.in-addr.arpa.": {
			PTR: []string{"mx.example.invalid."},
		},
	}
	resolver := mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &resolver,
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@[127.0.0.1]"})

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@[127.0.0.1]"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_FallbackMX(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	resolver := &mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"example.invalid.": {
				A: []string{"127.0.0.1"},
			},
		},
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    resolver,
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_BodyNonAtomic(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    resolver,
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	c := multipleErrs{
		errs: map[string]error{},
	}
	testutils.DoTestDeliveryNonAtomic(t, &c, &tgt, "test@example.com", []string{"test@example.invalid"})

	if err := c.errs["test@example.invalid"]; err != nil {
		t.Fatal(err)
	}

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_Abort(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err != nil {
		t.Fatal(err)
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_CommitWithoutBody(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err != nil {
		t.Fatal(err)
	}

	// Currently it does nothing, probably it should fail.
	if err := delivery.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_MAILFROMErr(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be.MailErr = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 2},
		Message:      "Hey",
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.AddRcpt("test@example.invalid")
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	fields := exterrors.Fields(err)
	if val, _ := fields["smtp_code"].(int); val != 550 {
		t.Errorf("Wrong smtp_code: %v", val)
	}
	if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); (val != exterrors.EnhancedCode{5, 1, 2}) {
		t.Errorf("Wrong smtp_enchcode: %v", val)
	}
	if val, _ := fields["smtp_msg"].(string); val != "mx.example.invalid. said: Hey" {
		t.Errorf("Wrong smtp_msg: %v", val)
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_NoMX(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err == nil {
		t.Fatal("Expected an error, got none")
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_NullMX(t *testing.T) {
	// Hang the test if it actually connects to the server to
	// deliver the message. Use of testutils.SMTPServer here
	// causes weird race conditions.
	tarpit := testutils.FailOnConn(t, "127.0.0.1"+addressSuffix)
	defer tarpit.Close()

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: ".", Pref: 10}},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err == nil {
		t.Fatal("Expected an error, got none")
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_Quarantined(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	meta := module.MsgMetadata{ID: "test..."}

	delivery, err := tgt.Start(&meta, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err != nil {
		t.Fatal(err)
	}

	meta.Quarantine = true

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	if err := delivery.Body(textproto.Header{}, body); err == nil {
		t.Fatal("Expected an error, got none")
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_MAILFROMErr_Repeated(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be.MailErr = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 2},
		Message:      "Hey",
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.AddRcpt("test@example.invalid")
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	fields := exterrors.Fields(err)
	if val, _ := fields["smtp_code"].(int); val != 550 {
		t.Errorf("Wrong smtp_code: %v", val)
	}
	if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); (val != exterrors.EnhancedCode{5, 1, 2}) {
		t.Errorf("Wrong smtp_enchcode: %v", val)
	}
	if val, _ := fields["smtp_msg"].(string); val != "mx.example.invalid. said: Hey" {
		t.Errorf("Wrong smtp_msg: %v", val)
	}

	err = delivery.AddRcpt("test2@example.invalid")
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	fields = exterrors.Fields(err)
	if val, _ := fields["smtp_code"].(int); val != 550 {
		t.Errorf("Wrong smtp_code: %v", val)
	}
	if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); (val != exterrors.EnhancedCode{5, 1, 2}) {
		t.Errorf("Wrong smtp_enchcode: %v", val)
	}
	if val, _ := fields["smtp_msg"].(string); val != "mx.example.invalid. said: Hey" {
		t.Errorf("Wrong smtp_msg: %v", val)
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_RcptErr(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be.RcptErr = map[string]error{
		"test@example.invalid": &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 1, 2},
			Message:      "Hey",
		},
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.AddRcpt("test@example.invalid")
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	fields := exterrors.Fields(err)
	if val, _ := fields["smtp_code"].(int); val != 550 {
		t.Errorf("Wrong smtp_code: %v", val)
	}
	if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); (val != exterrors.EnhancedCode{5, 1, 2}) {
		t.Errorf("Wrong smtp_enchcode: %v", val)
	}
	if val, _ := fields["smtp_msg"].(string); val != "mx.example.invalid. said: Hey" {
		t.Errorf("Wrong smtp_msg: %v", val)
	}

	// It should be possible to, however, add another recipient and continue
	// delivery as if nothing happened.
	if err := delivery.AddRcpt("test2@example.invalid"); err != nil {
		t.Fatal(err)
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	if err := delivery.Body(hdr, body); err != nil {
		t.Fatal(err)
	}

	if err := delivery.Commit(); err != nil {
		t.Fatal(err)
	}

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test2@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_DownMX(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{
				{Host: "mx1.example.invalid.", Pref: 20},
				{Host: "mx2.example.invalid.", Pref: 10},
			},
		},
		"mx1.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx2.example.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_AllMXDown(t *testing.T) {
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{
				{Host: "mx1.example.invalid.", Pref: 20},
				{Host: "mx2.example.invalid.", Pref: 10},
			},
		},
		"mx1.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx2.example.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}
}

func TestRemoteDelivery_Split(t *testing.T) {
	be1, srv1 := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)
	be2, srv2 := testutils.SMTPServer(t, "127.0.0.2"+addressSuffix)
	defer srv2.Close()
	defer testutils.CheckSMTPConnLeak(t, srv2)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"example2.invalid.": {
			MX: []net.MX{{Host: "mx.example2.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx.example2.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid", "test@example2.invalid"})

	if len(be1.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be1.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}

	if len(be2.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg = be2.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example2.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_Split_Fail(t *testing.T) {
	be1, srv1 := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)
	be2, srv2 := testutils.SMTPServer(t, "127.0.0.2"+addressSuffix)
	defer srv2.Close()
	defer testutils.CheckSMTPConnLeak(t, srv2)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"example2.invalid.": {
			MX: []net.MX{{Host: "mx.example2.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx.example2.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be1.RcptErr = map[string]error{
		"test@example.invalid": &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 1, 2},
			Message:      "Hey",
		},
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.AddRcpt("test@example.invalid")
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	// It should be possible to, however, add another recipient and continue
	// delivery as if nothing happened.
	if err := delivery.AddRcpt("test@example2.invalid"); err != nil {
		t.Fatal(err)
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	if err := delivery.Body(hdr, body); err != nil {
		t.Fatal(err)
	}

	if err := delivery.Commit(); err != nil {
		t.Fatal(err)
	}

	if len(be2.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be2.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example2.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestRemoteDelivery_BodyErr(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be.DataErr = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 2},
		Message:      "Hey",
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	err = delivery.AddRcpt("test@example.invalid")
	if err != nil {
		t.Fatal(err)
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	if err := delivery.Body(hdr, body); err == nil {
		t.Fatal("expected an error, got none")
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_Split_BodyErr(t *testing.T) {
	be1, srv1 := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)
	_, srv2 := testutils.SMTPServer(t, "127.0.0.2"+addressSuffix)
	defer srv2.Close()
	defer testutils.CheckSMTPConnLeak(t, srv2)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"example2.invalid.": {
			MX: []net.MX{{Host: "mx.example2.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx.example2.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be1.DataErr = &smtp.SMTPError{
		Code:         421,
		EnhancedCode: smtp.EnhancedCode{4, 1, 2},
		Message:      "Hey",
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := delivery.AddRcpt("test@example2.invalid"); err != nil {
		t.Fatal(err)
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	err = delivery.Body(hdr, body)
	if err == nil {
		t.Fatal(err)
	}

	fields := exterrors.Fields(err)
	if code, _ := fields["smtp_code"].(int); code != 451 {
		t.Errorf("Wrong smtp_code: %v", code)
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_Split_BodyErr_NonAtomic(t *testing.T) {
	be1, srv1 := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)
	_, srv2 := testutils.SMTPServer(t, "127.0.0.2"+addressSuffix)
	defer srv2.Close()
	defer testutils.CheckSMTPConnLeak(t, srv2)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"example2.invalid.": {
			MX: []net.MX{{Host: "mx.example2.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx.example2.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	be1.DataErr = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 2},
		Message:      "Hey",
	}
	checkErr := func(err error) {
		t.Helper()

		t.Logf("%#v", err)

		if err == nil {
			t.Error("Expected an error, got none")
			return
		}

		fields := exterrors.Fields(err)
		if val, _ := fields["smtp_code"].(int); val != 550 {
			t.Errorf("Wrong smtp_code: %v", val)
		}
		if val, _ := fields["smtp_enchcode"].(exterrors.EnhancedCode); (val != exterrors.EnhancedCode{5, 1, 2}) {
			t.Errorf("Wrong smtp_enchcode: %v", val)
		}
		if val, _ := fields["smtp_msg"].(string); !strings.HasSuffix(val, "Hey") {
			t.Errorf("Wrong smtp_msg: %v", val)
		}
	}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	delivery, err := tgt.Start(&module.MsgMetadata{ID: "test..."}, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if err := delivery.AddRcpt("test@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := delivery.AddRcpt("test2@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := delivery.AddRcpt("test@example2.invalid"); err != nil {
		t.Fatal(err)
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	body := buffer.MemoryBuffer{Slice: []byte("foobar\r\n")}
	c := multipleErrs{
		errs: map[string]error{},
	}
	delivery.(module.PartialDelivery).BodyNonAtomic(&c, hdr, body)

	checkErr(c.errs["test@example.invalid"])
	checkErr(c.errs["test2@example.invalid"])
	if err := c.errs["test@example2.invalid"]; err != nil {
		t.Errorf("Unexpected error for non-failing connection: %v", err)
	}

	if err := delivery.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDelivery_RequireTLS_Missing(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		requireTLS:  true,
		Log:         testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Errorf("expected an error, got none")
	}
}

func TestRemoteDelivery_RequireTLS_Present(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1"+addressSuffix)
	defer srv.Close()
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.com",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		requireTLS:  true,
		tlsConfig:   clientCfg,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})

	if len(be.Messages) != 1 {
		t.Fatalf("Expected to receive a message, got none")
	}
	msg := be.Messages[0]
	if msg.From != "test@example.com" {
		t.Errorf("Wrong MAIL FROM: %v", msg.From)
	}
	if !reflect.DeepEqual(msg.To, []string{"test@example.invalid"}) {
		t.Errorf("Wrong RCPT TO: %v", msg.To)
	}
	if string(msg.Data) != testutils.DeliveryData {
		t.Errorf("Wrong DATA payload: %v", string(msg.Data))
	}
}

func TestMain(m *testing.M) {
	remoteSmtpPort := flag.String("test.smtpport", "random", "(maddy) SMTP port to use for connections in tests")
	flag.Parse()

	if *remoteSmtpPort == "random" {
		rand.Seed(time.Now().UnixNano())
		*remoteSmtpPort = strconv.Itoa(rand.Intn(65536-10000) + 10000)
	}

	addressSuffix = ":" + *remoteSmtpPort
	os.Exit(m.Run())
}
