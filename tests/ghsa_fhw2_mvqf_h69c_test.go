//go:build integration
// +build integration

package tests_test

import (
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/tests"
)

func TestGHSAFHW2MVQFH69CMalformedFromDefaultAccepted(tt *testing.T) {
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		"_dmarc.bank.example.": {
			TXT: []string{"v=DMARC1; p=reject"},
		},
	})
	t.Port("smtp")
	t.Config(`
		hostname mx.maddy.test
		tls off

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			dmarc yes
			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("client.maddy.test", nil, nil)
	conn.Writeln("MAIL FROM:<sender@attacker.example>")
	conn.ExpectPattern("250 *")
	conn.Writeln("RCPT TO:<recipient@maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("DATA")
	conn.ExpectPattern("354 *")
	conn.Writeln("From: Bank Example CEO <ceo@bank.example")
	conn.Writeln("To: <recipient@maddy.test>")
	conn.Writeln("Subject: Important message")
	conn.Writeln("")
	conn.Writeln("Please review this message.")
	conn.Writeln(".")
	conn.ExpectPattern("250 2.0.0 *")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestGHSAFHW2MVQFH69CMalformedFromRejectedWhenEnabled(tt *testing.T) {
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{})
	t.Port("smtp")
	t.Config(`
		hostname mx.maddy.test
		tls off

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			dmarc yes
			dmarc_reject_invalid_from yes
			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("client.maddy.test", nil, nil)
	conn.Writeln("MAIL FROM:<sender@attacker.example>")
	conn.ExpectPattern("250 *")
	conn.Writeln("RCPT TO:<recipient@maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("DATA")
	conn.ExpectPattern("354 *")
	conn.Writeln("From: Bank Example CEO <ceo@bank.example")
	conn.Writeln("To: <recipient@maddy.test>")
	conn.Writeln("Subject: Important message")
	conn.Writeln("")
	conn.Writeln("Please review this message.")
	conn.Writeln(".")
	conn.ExpectPattern("550 5.7.1 *")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}
