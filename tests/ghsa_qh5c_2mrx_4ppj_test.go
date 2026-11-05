//go:build integration

package tests_test

import (
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/tests"
)

func TestGHSA_QH5C_2MRX_4PPJ_DMARC_MixedCaseBypass(tt *testing.T) {
	const (
		f1AttackerSPF = "v=spf1 +all"
		f1VictimDMARC = "v=DMARC1; p=reject; adkim=r; aspf=r"
	)

	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		"attacker.co.uk.":        {TXT: []string{f1AttackerSPF}},
		"attacker.co.UK.":        {TXT: []string{f1AttackerSPF}},
		"_dmarc.barclays.co.uk.": {TXT: []string{f1VictimDMARC}},
		"_dmarc.barclays.co.UK.": {TXT: []string{f1VictimDMARC}},
	})
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no

			dmarc yes
			check {
				spf {
					none_action ignore
					fail_action ignore
					softfail_action ignore
					permerr_action ignore
					temperr_action ignore
				}
				dkim {
					no_sig_action ignore
					broken_sig_action ignore
				}
			}

			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	// A fixed build rejects this (550). We assert the secure outcome, so the
	// test fails loudly on the vulnerable build, pointing at the accepted spoof.
	conn := t.Conn4("127.0.0.1", "smtp")
	defer conn.Close()

	conn.SMTPNegotation("localhost", nil, nil)
	conn.Writeln("MAIL FROM:<attacker@attacker.co.UK>")
	conn.ExpectPattern("250 *")
	conn.Writeln("RCPT TO:<postmaster@maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("DATA")
	conn.ExpectPattern("354 *")
	conn.Writeln("From: CEO <ceo@barclays.co.UK>")
	conn.Writeln("To: <postmaster@maddy.test>")
	conn.Writeln("Subject: quarterly notice")
	conn.Writeln("")
	conn.Writeln("hello")
	conn.Writeln(".")

	final := conn.ExpectPattern("*")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")

	if len(final) >= 3 && final[0:3] == "250" {
		tt.Fatalf("BYPASS CONFIRMED: mixed-case spoof of barclays.co.UK "+
			"(p=reject) was ACCEPTED by maddy (%q); a fixed build rejects it with 550", final)
	}
}
