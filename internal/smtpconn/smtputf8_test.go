package smtpconn

import (
	"context"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/testutils"
)

func doTestDelivery(t *testing.T, conn *C, from string, to []string, opts smtp.MailOptions) error {
	t.Helper()

	if err := conn.Mail(context.Background(), from, opts); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := conn.Rcpt(context.Background(), rcpt); err != nil {
			return err
		}
	}

	hdr := textproto.Header{}
	hdr.Add("B", "2")
	hdr.Add("A", "1")
	if err := conn.Data(context.Background(), hdr, strings.NewReader("foobar\n")); err != nil {
		return err
	}

	return nil
}

func TestSMTPUTF8_Sender_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@тест.example.org", []string{"test@example.invalid"},
		smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "test@xn--e1aybc.example.org", []string{"test@example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestSMTPUTF8_Rcpt_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@example.org", []string{"test@тест.example.invalid"},
		smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "test@example.org", []string{"test@xn--e1aybc.example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestSMTPUTF8_Sender_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "тест@example.org", []string{"test@example.invalid"},
		smtp.MailOptions{UTF8: true})
	testutils.CheckSMTPErr(t, err, 550, exterrors.EnhancedCode{5, 6, 7},
		"SMTPUTF8 is unsupported, cannot convert sender address")
}

func TestSMTPUTF8_Rcpt_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@example.org", []string{"тест@example.invalid"}, smtp.MailOptions{UTF8: true})
	testutils.CheckSMTPErr(t, err, 553, exterrors.EnhancedCode{5, 6, 7},
		"SMTPUTF8 is unsupported, cannot convert recipient address")
}

func TestSMTPUTF8_Sender_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@тест.org", []string{"test@example.invalid"}, smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "test@тест.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestSMTPUTF8_Rcpt_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@example.org", []string{"test@тест.example.invalid"},
		smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "test@example.org", []string{"test@тест.example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestSMTPUTF8_Sender_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "тест@example.org", []string{"test@example.invalid"},
		smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "тест@example.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestSMTPUTF8_Rcpt_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	c := New()
	c.Log = testutils.Logger(t, "smtp_downstream")
	if err := c.Connect(context.Background(), config.Endpoint{
		Scheme: "tcp",
		Host:   "127.0.0.1",
		Port:   testPort,
	}); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err := doTestDelivery(t, c, "test@example.org", []string{"тест@example.invalid"},
		smtp.MailOptions{UTF8: true})
	if err != nil {
		t.Fatal(err)
	}

	be.CheckMsg(t, 0, "test@example.org", []string{"тест@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}
