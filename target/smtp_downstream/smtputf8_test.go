package smtp_downstream

import (
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/testutils"
)

func TestDownstreamDelivery_EHLO_ALabel(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod, err := NewDownstream("", "", nil, []string{"tcp://127.0.0.1:" + testPort})
	if err != nil {
		t.Fatal(err)
	}
	if err := mod.Init(config.NewMap(nil, &config.Node{
		Children: []config.Node{
			{
				Name: "hostname",
				Args: []string{"тест.invalid"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}

	tgt := mod.(*Downstream)
	tgt.log = testutils.Logger(t, "remote")

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})

	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
	if be.Messages[0].State.Hostname != "xn--e1aybc.invalid" {
		t.Error("target/remote should use use Punycode in EHLO")
	}
}

func TestDownstreamDelivery_Sender_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@тест.example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@xn--e1aybc.example.org", []string{"test@example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestDownstreamDelivery_Rcpt_UTF8_Punycode(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@example.org", []string{"test@тест.example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"test@xn--e1aybc.example.invalid"})
	if be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is set")
	}
}

func TestDownstreamDelivery_Sender_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	_, err := testutils.DoTestDeliveryErrMeta(t, mod, "тест@example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})
	testutils.CheckSMTPErr(t, err, 550, exterrors.EnhancedCode{5, 6, 7},
		"Downstream does not support SMTPUTF8, can not convert sender address")
}

func TestDownstreamDelivery_Rcpt_UTF8_Reject(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = false
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	_, err := testutils.DoTestDeliveryErrMeta(t, mod, "test@example.org", []string{"тест@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})
	testutils.CheckSMTPErr(t, err, 553, exterrors.EnhancedCode{5, 6, 7},
		"Downstream does not support SMTPUTF8, can not convert recipient address")
}

func TestDownstreamDelivery_Sender_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@тест.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@тест.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestDownstreamDelivery_Rcpt_UTF8_Domain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@example.org", []string{"test@тест.example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"test@тест.example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestDownstreamDelivery_Sender_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "тест@example.org", []string{"test@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "тест@example.org", []string{"test@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}

func TestDownstreamDelivery_Rcpt_UTF8_Username(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	srv.EnableSMTPUTF8 = true
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@example.org", []string{"тест@example.invalid"}, &module.MsgMetadata{
		SMTPOpts: smtp.MailOptions{
			UTF8: true,
		},
	})

	be.CheckMsg(t, 0, "test@example.org", []string{"тест@example.invalid"})
	if !be.Messages[0].Opts.UTF8 {
		t.Error("SMTPUTF8 flag is not set")
	}
}
