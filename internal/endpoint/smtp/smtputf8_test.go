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

package smtp

import (
	"strings"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestSMTPUTF8_MangleStatusMessage(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, []module.Check{
		&testutils.Check{
			ConnRes: module.CheckResult{
				Reason: &exterrors.SMTPError{
					Code:    523,
					Message: "Hey 凱凱",
				},
				Reject: true,
			},
		},
	}, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = cl.Mail("sender@example.org", nil)
	if err == nil {
		t.Fatal("Expected an error, got none")
	}
	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatal("Non-SMTPError returned")
	}

	if smtpErr.Code != 523 {
		t.Fatal("Wrong SMTP code:", smtpErr.Code)
	}
	if !strings.HasPrefix(smtpErr.Message, "Hey ??") {
		t.Fatal("Wrong SMTP message:", smtpErr.Message)
	}
}

func TestSMTP_RejectNonASCIIFrom(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsg(t, cl, "ѣ@example.org", []string{"rcpt@example.com"}, testMsg)

	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatal("Non-SMTPError returned")
	}
	if smtpErr.Code != 550 {
		t.Fatal("Wrong SMTP code:", smtpErr.Code)
	}
	if smtpErr.EnhancedCode != (smtp.EnhancedCode{5, 6, 7}) {
		t.Fatal("Wrong SMTP ench. code:", smtpErr.EnhancedCode)
	}
}

func TestSMTPUTF8_NormalizeCaseFoldFrom(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsgOpts(t, cl, "foo@E\u0301.example.org", []string{"rcpt@example.com"}, &smtp.MailOptions{
		UTF8: true,
	}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	testutils.CheckMsgID(t, &msg, "foo@é.example.org", []string{"rcpt@example.com"}, "")
}

func TestSMTP_RejectNonASCIIRcpt(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsg(t, cl, "x@example.org", []string{"ѣ@example.org"}, testMsg)

	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatal("Non-SMTPError returned")
	}
	if smtpErr.Code != 553 {
		t.Fatal("Wrong SMTP code:", smtpErr.Code)
	}
	if smtpErr.EnhancedCode != (smtp.EnhancedCode{5, 6, 7}) {
		t.Fatal("Wrong SMTP ench. code:", smtpErr.EnhancedCode)
	}
}

func TestSMTPUTF8_NormalizeCaseFoldRcpt(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsgOpts(t, cl, "x@example.org", []string{"foo@E\u0301.example.org"}, &smtp.MailOptions{
		UTF8: true,
	}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	testutils.CheckMsgID(t, &msg, "x@example.org", []string{"foo@é.example.org"}, "")
}

func TestSMTPUTF8_NoMangleStatusMessage(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, []module.Check{
		&testutils.Check{
			ConnRes: module.CheckResult{
				Reason: &exterrors.SMTPError{
					Code:    523,
					Message: "Hey 凱凱",
				},
				Reject: true,
			},
		},
	}, nil)
	endp.deferServerReject = false
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = cl.Mail("sender@example.org", &smtp.MailOptions{
		UTF8: true,
	})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}
	smtpErr, ok := err.(*smtp.SMTPError)
	if !ok {
		t.Fatal("Non-SMTPError returned")
	}

	if smtpErr.Code != 523 {
		t.Fatal("Wrong SMTP code:", smtpErr.Code)
	}
	if !strings.HasPrefix(smtpErr.Message, "Hey 凱凱") {
		t.Fatal("Wrong SMTP message:", smtpErr.Message)
	}
}

func TestSMTPUTF8_Received_EHLO_ALabel(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	if err := cl.Hello("凱凱.invalid"); err != nil {
		t.Fatal(err)
	}

	err = submitMsg(t, cl, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	msgID := testutils.CheckMsgID(t, &msg, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, "")

	receivedPrefix := `from xn--y9qa.invalid (mx.example.org [127.0.0.1]) by mx.example.com (envelope-sender <sender@example.org>) with ESMTP id ` + msgID

	if !strings.HasPrefix(msg.Header.Get("Received"), receivedPrefix) {
		t.Error("Wrong Received contents:", msg.Header.Get("Received"))
	}
}

func TestSMTPUTF8_Received_rDNS_ALabel(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	endp.resolver.(*mockdns.Resolver).Zones["1.0.0.127.in-addr.arpa."] = mockdns.Zone{
		PTR: []string{"凱凱.invalid."},
	}

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsg(t, cl, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	msgID := testutils.CheckMsgID(t, &msg, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, "")

	receivedPrefix := `from mx.example.org (xn--y9qa.invalid [127.0.0.1]) by mx.example.com (envelope-sender <sender@example.org>) with ESMTP id ` + msgID

	if !strings.HasPrefix(msg.Header.Get("Received"), receivedPrefix) {
		t.Error("Wrong Received contents:", msg.Header.Get("Received"))
	}
}

func TestSMTPUTF8_Received_rDNS_ULabel(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	endp.resolver.(*mockdns.Resolver).Zones["1.0.0.127.in-addr.arpa."] = mockdns.Zone{
		PTR: []string{"凱凱.invalid."},
	}

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = submitMsgOpts(t, cl, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, &smtp.MailOptions{
		UTF8: true,
	}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	msgID := testutils.CheckMsgID(t, &msg, "sender@example.org", []string{"rcpt1@example.com", "rcpt2@example.com"}, "")

	receivedPrefix := `from mx.example.org (凱凱.invalid [127.0.0.1]) by mx.example.com (envelope-sender <sender@example.org>) with UTF8ESMTP id ` + msgID

	if !strings.HasPrefix(msg.Header.Get("Received"), receivedPrefix) {
		t.Error("Wrong Received contents:", msg.Header.Get("Received"))
	}
}

func TestSMTPUTF8_Received_EHLO_ULabel(t *testing.T) {
	tgt := testutils.Target{}
	endp := testEndpoint(t, "smtp", nil, &tgt, nil, nil)
	defer endp.Close()
	defer testutils.WaitForConnsClose(t, endp.serv)

	cl, err := smtp.Dial("127.0.0.1:" + testPort)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	if err := cl.Hello("凱凱.invalid"); err != nil {
		t.Fatal(err)
	}

	err = submitMsgOpts(t, cl, "sender@example.org", []string{"rcpt@example.com"}, &smtp.MailOptions{
		UTF8: true,
	}, testMsg)
	if err != nil {
		t.Fatal(err)
	}

	if len(tgt.Messages) != 1 {
		t.Fatal("Expected a message, got", len(tgt.Messages))
	}
	msg := tgt.Messages[0]
	msgID := testutils.CheckMsgID(t, &msg, "sender@example.org", []string{"rcpt@example.com"}, "")

	// Also, 'with UTF8ESMTP'.
	receivedPrefix := `from 凱凱.invalid (mx.example.org [127.0.0.1]) by mx.example.com (envelope-sender <sender@example.org>) with UTF8ESMTP id ` + msgID

	if !strings.HasPrefix(msg.Header.Get("Received"), receivedPrefix) {
		t.Error("Wrong Received contents:", msg.Header.Get("Received"))
	}
}
