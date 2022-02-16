/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package smtp_downstream

import (
	"errors"
	"flag"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
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
		log: testutils.Logger(t, "target.smtp"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
}

func TestDownstreamDelivery_LMTP(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort, func(srv *smtp.Server) {
		srv.LMTP = true
	})
	be.LMTPDataErr = []error{
		nil,
		&smtp.SMTPError{
			Code:    501,
			Message: "nop",
		},
	}
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
		modName: "target.lmtp",
		lmtp:    true,
		log:     testutils.Logger(t, "lmtp_downstream"),
	}

	sc := make(statusCollector)

	testutils.DoTestDeliveryNonAtomic(t, &sc, mod, "test@example.invalid", []string{"rcpt1@example.invalid", "rcpt2@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt1@example.invalid", "rcpt2@example.invalid"})

	if len(sc) != 2 {
		t.Fatal("Two statuses should be set")
	}
	if err := sc["rcpt1@example.invalid"]; err != nil {
		t.Fatal("Unexpected error for rcpt1:", err)
	}
	if sc["rcpt2@example.invalid"] == nil {
		t.Fatal("Expected an error for rcpt2")
	}
	var rcptErr *exterrors.SMTPError
	if !errors.As(sc["rcpt2@example.invalid"], &rcptErr) {
		t.Fatalf("Not SMTPError: %T", rcptErr)
	}
	if rcptErr.Code != 501 {
		t.Fatal("Wrong SMTP code:", rcptErr.Code)
	}
}

func TestDownstreamDelivery_LMTP_ErrorCoerce(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort, func(srv *smtp.Server) {
		srv.LMTP = true
	})
	be.LMTPDataErr = []error{
		nil,
		&smtp.SMTPError{
			Code:    501,
			Message: "nop",
		},
	}
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
		modName: "target.lmtp",
		lmtp:    true,
		log:     testutils.Logger(t, "lmtp_downstream"),
	}

	_, err := testutils.DoTestDeliveryErr(t, mod, "test@example.invalid", []string{"rcpt1@example.invalid", "rcpt2@example.invalid"})
	if err == nil {
		t.Error("expected failure")
	}
}

type statusCollector map[string]error

func (sc *statusCollector) SetStatus(rcptTo string, err error) {
	(*sc)[rcptTo] = err
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
		log: testutils.Logger(t, "target.smtp"),
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
		log: testutils.Logger(t, "target.smtp"),
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
		log:             testutils.Logger(t, "target.smtp"),
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
		log:             testutils.Logger(t, "target.smtp"),
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
		log:             testutils.Logger(t, "target.smtp"),
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
		log:             testutils.Logger(t, "target.smtp"),
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
		log:             testutils.Logger(t, "target.smtp"),
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
