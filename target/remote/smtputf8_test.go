package remote

import (
	"net"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

func TestRemoteDelivery_EHLO_ALabel(t *testing.T) {
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

	mod, err := New("", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.Init(config.NewMap(nil, &config.Node{
		Children: []config.Node{
			{
				Name: "authenticate_mx",
				Args: []string{"off"},
			},
			{
				Name: "hostname",
				Args: []string{"тест.invalid"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}

	tgt := mod.(*Target)
	tgt.resolver = &mockdns.Resolver{Zones: zones}
	tgt.dialer = resolver.Dial
	tgt.extResolver = nil
	tgt.Log = testutils.Logger(t, "remote")

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})

	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
	if be.Messages[0].State.Hostname != "xn--e1aybc.invalid" {
		t.Error("target/remote should use use Punycode in EHLO")
	}
}

func TestRemoteDelivery_Sender_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = false // No SMTPUTF8 support!
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
		hostname:    "example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "test@тест.example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@xn--e1aybc.example.org", []string{"test@example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestRemoteDelivery_Rcpt_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = false // No SMTPUTF8 support!
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"тест.example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "test@example.org", []string{"test@тест.example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"test@xn--e1aybc.example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestRemoteDelivery_Sender_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = false // No SMTPUTF8 support!
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
		hostname:    "example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErrMeta(t, &tgt, "тест@example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})
	testutils.CheckSMTPErr(t, err, 550, exterrors.EnhancedCode{5, 6, 7},
		"Remote MX does not support SMTPUTF8, can not convert sender address")
}

func TestRemoteDelivery_Rcpt_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = false // No SMTPUTF8 support!
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
		hostname:    "example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErrMeta(t, &tgt, "test@example.org", []string{"тест@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})
	testutils.CheckSMTPErr(t, err, 553, exterrors.EnhancedCode{5, 6, 7},
		"Remote MX does not support SMTPUTF8, can not convert recipient address")
}

func TestRemoteDelivery_Sender_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = true // SMTPUTF8 support!
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
		hostname:    "mx.example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "test@тест.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@тест.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestRemoteDelivery_Rcpt_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = true // SMTPUTF8 support!
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"тест.example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:        "remote",
		hostname:    "mx.example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "test@example.org", []string{"test@тест.example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"test@тест.example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestRemoteDelivery_Sender_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = true // SMTPUTF8 support!
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
		hostname:    "mx.example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "тест@example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "тест@example.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestRemoteDelivery_Rcpt_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1"+addressSuffix)
	srv.EnableSMTPUTF8 = true // SMTPUTF8 support!
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
		hostname:    "mx.example.org",
		resolver:    &mockdns.Resolver{Zones: zones},
		dialer:      resolver.Dial,
		extResolver: nil,
		Log:         testutils.Logger(t, "remote"),
	}

	testutils.DoTestDeliveryMeta(t, &tgt, "test@example.org", []string{"тест@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"тест@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}
