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
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func testSaslFactory(t *testing.T, args ...string) saslClientFactory {
	factory, err := saslAuthDirective(&config.Map{}, config.Node{
		Name: "auth",
		Args: args,
	})
	if err != nil {
		t.Fatal(err)
	}
	return factory.(saslClientFactory)
}

func TestSASL_Plain(t *testing.T) {
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
		saslFactory: testSaslFactory(t, "plain", "test", "testpass"),
		log:         testutils.Logger(t, "target.smtp"),
	}

	testutils.DoTestDelivery(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
	if be.Messages[0].AuthUser != "test" {
		t.Errorf("Wrong AuthUser: %v", be.Messages[0].AuthUser)
	}
	if be.Messages[0].AuthPass != "testpass" {
		t.Errorf("Wrong AuthPass: %v", be.Messages[0].AuthPass)
	}
}

func TestSASL_Plain_AuthFail(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	be.AuthErr = &smtp.SMTPError{
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
		saslFactory: testSaslFactory(t, "plain", "test", "testpass"),
		log:         testutils.Logger(t, "target.smtp"),
	}

	_, err := testutils.DoTestDeliveryErr(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
}

func TestSASL_Forward(t *testing.T) {
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
		saslFactory: testSaslFactory(t, "forward"),
		log:         testutils.Logger(t, "target.smtp"),
	}

	testutils.DoTestDeliveryMeta(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"}, &module.MsgMetadata{
		Conn: &module.ConnState{
			AuthUser:     "test",
			AuthPassword: "testpass",
		},
	})
	be.CheckMsg(t, 0, "test@example.invalid", []string{"rcpt@example.invalid"})
	if be.Messages[0].AuthUser != "test" {
		t.Errorf("Wrong AuthUser: %v", be.Messages[0].AuthUser)
	}
	if be.Messages[0].AuthPass != "testpass" {
		t.Errorf("Wrong AuthPass: %v", be.Messages[0].AuthPass)
	}
}

func TestSASL_Forward_NoCreds(t *testing.T) {
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
		saslFactory: testSaslFactory(t, "forward"),
		log:         testutils.Logger(t, "target.smtp"),
	}

	_, err := testutils.DoTestDeliveryErr(t, mod, "test@example.invalid", []string{"rcpt@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
}
