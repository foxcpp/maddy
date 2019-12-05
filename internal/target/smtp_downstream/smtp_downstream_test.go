package smtp_downstream

import (
	"flag"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/testutils"
)

var testPort string

func TestDownstreamDelivery(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	tarpit := testutils.FailOnConn(t, "127.0.0.2:"+testPort)
	defer tarpit.Close()

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tcp",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
			{
				Scheme: "tcp",
				Host:   "127.0.0.2",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
}

func TestDownstreamDelivery_Fallback(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.2:"+testPort)
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
			{
				Scheme: "tcp",
				Host:   "127.0.0.2",
				Port:   testPort,
			},
		},
		log: testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
}

func TestDownstreamDelivery_MAILErr(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	be.MailErr = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 2},
		Message:      "Hey",
	}

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

	_, err := testutils.DoTestDeliveryErr(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	testutils.CheckSMTPErr(t, err, 550, exterrors.EnhancedCode{5, 1, 2}, "Hey")
}

func TestDownstreamDelivery_AttemptTLS(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+testPort)
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
		tlsConfig:       *clientCfg.Clone(),
		attemptStartTLS: true,
		log:             testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
	if !be.Messages[0].State.TLS.HandshakeComplete {
		t.Error("Expected TLS to be used, but it was not")
	}
}

func TestDownstreamDelivery_AttemptTLS_Fallback(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
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
		attemptStartTLS: true,
		log:             testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
}

func TestDownstreamDelivery_RequireTLS(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+testPort)
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
		tlsConfig:       *clientCfg.Clone(),
		attemptStartTLS: true,
		requireTLS:      true,
		log:             testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
	if !be.Messages[0].State.TLS.HandshakeComplete {
		t.Error("Expected TLS to be used, but it was not")
	}
}

func TestDownstreamDelivery_RequireTLS_Implicit(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerTLS(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	mod := &Downstream{
		hostname: "mx.example.invalid",
		endpoints: []config.Endpoint{
			{
				Scheme: "tls",
				Host:   "127.0.0.1",
				Port:   testPort,
			},
		},
		tlsConfig:       *clientCfg.Clone(),
		attemptStartTLS: true,
		requireTLS:      true,
		log:             testutils.Logger(t, "smtp_downstream"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
	if !be.Messages[0].State.TLS.HandshakeComplete {
		t.Error("Expected TLS to be used, but it was not")
	}
}

func TestDownstreamDelivery_RequireTLS_Fail(t *testing.T) {
	_, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
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
		attemptStartTLS: true,
		requireTLS:      true,
		log:             testutils.Logger(t, "smtp_downstream"),
	}

	_, err := testutils.DoTestDeliveryErr(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
}

func TestMain(m *testing.M) {
	remoteSmtpPort := flag.String("test.smtpport", "random", "(maddy) SMTP port to use for connections in tests")
	flag.Parse()

	if *remoteSmtpPort == "random" {
		rand.Seed(time.Now().UnixNano())
		*remoteSmtpPort = strconv.Itoa(rand.Intn(65536-10000) + 10000)
	}

	testPort = *remoteSmtpPort
	os.Exit(m.Run())
}
