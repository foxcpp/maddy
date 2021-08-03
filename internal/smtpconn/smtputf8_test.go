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

package smtpconn

import (
	"context"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
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
	return conn.Data(context.Background(), hdr, strings.NewReader("foobar\n"))
}

func TestSMTPUTF8(t *testing.T) {
	type test struct {
		clientSender string
		clientRcpt   string

		serverUTF8   bool
		serverSender string
		serverRcpt   string

		expectUTF8 bool
		expectErr  *exterrors.SMTPError
	}
	check := func(case_ test) {
		t.Helper()

		be, srv := testutils.SMTPServer(t, "127.0.0.1:"+testPort)
		srv.EnableSMTPUTF8 = case_.serverUTF8
		defer srv.Close()
		defer testutils.CheckSMTPConnLeak(t, srv)

		c := New()
		c.Log = testutils.Logger(t, "target.smtp")
		if _, err := c.Connect(context.Background(), config.Endpoint{
			Scheme: "tcp",
			Host:   "127.0.0.1",
			Port:   testPort,
		}, false, nil); err != nil {
			t.Fatal(err)
		}
		defer c.Close()

		err := doTestDelivery(t, c, case_.clientSender, []string{case_.clientRcpt},
			smtp.MailOptions{UTF8: true})
		if err != nil {
			if case_.expectErr == nil {
				t.Error("Unexpected failure")
			} else {
				testutils.CheckSMTPErr(t, err, case_.expectErr.Code, case_.expectErr.EnhancedCode, case_.expectErr.Message)
			}
			return
		} else if case_.expectErr != nil {
			t.Error("Unexpected success")
		}

		be.CheckMsg(t, 0, case_.serverSender, []string{case_.serverRcpt})
		if be.Messages[0].Opts.UTF8 != case_.expectUTF8 {
			t.Errorf("expectUTF8 = %v, SMTPUTF8 = %v", case_.expectErr, be.Messages[0].Opts.UTF8)
		}
	}

	check(test{
		clientSender: "test@тест.example.org",
		clientRcpt:   "test@example.invalid",
		serverSender: "test@xn--e1aybc.example.org",
		serverRcpt:   "test@example.invalid",
	})
	check(test{
		clientSender: "test@example.org",
		clientRcpt:   "test@тест.example.invalid",
		serverSender: "test@example.org",
		serverRcpt:   "test@xn--e1aybc.example.invalid",
	})
	check(test{
		clientSender: "тест@example.org",
		clientRcpt:   "test@example.invalid",
		serverUTF8:   false,
		expectErr: &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
			Message:      "SMTPUTF8 is unsupported, cannot convert sender address",
		},
	})
	check(test{
		clientSender: "test@example.org",
		clientRcpt:   "тест@example.invalid",
		serverUTF8:   false,
		expectErr: &exterrors.SMTPError{
			Code:         553,
			EnhancedCode: exterrors.EnhancedCode{5, 6, 7},
			Message:      "SMTPUTF8 is unsupported, cannot convert recipient address",
		},
	})
	check(test{
		clientSender: "test@тест.org",
		clientRcpt:   "test@example.invalid",
		serverSender: "test@тест.org",
		serverRcpt:   "test@example.invalid",
		serverUTF8:   true,
		expectUTF8:   true,
	})
	check(test{
		clientSender: "test@example.org",
		clientRcpt:   "test@тест.example.invalid",
		serverSender: "test@example.org",
		serverRcpt:   "test@тест.example.invalid",
		serverUTF8:   true,
		expectUTF8:   true,
	})
	check(test{
		clientSender: "тест@example.org",
		clientRcpt:   "test@example.invalid",
		serverSender: "тест@example.org",
		serverRcpt:   "test@example.invalid",
		serverUTF8:   true,
		expectUTF8:   true,
	})
	check(test{
		clientSender: "test@example.org",
		clientRcpt:   "тест@example.invalid",
		serverSender: "test@example.org",
		serverRcpt:   "тест@example.invalid",
		serverUTF8:   true,
		expectUTF8:   true,
	})
}
