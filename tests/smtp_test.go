//go:build integration
// +build integration

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

package tests_test

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/tests"
)

func TestCheckRequireTLS(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls self_signed

			defer_sender_reject no

			check {
				require_tls
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)
	conn.Writeln("MAIL FROM:<testing@two.maddy.test>")
	conn.ExpectPattern("550 5.7.1 *")
	conn.Writeln("STARTTLS")
	conn.ExpectPattern("220 *")
	conn.TLS()
	conn.SMTPNegotation("localhost", nil, nil)
	conn.Writeln("MAIL FROM:<testing@two.maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestCheckSPF(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		"none.maddy.test.": {
			TXT: []string{},
		},
		"pass.maddy.test.": {
			TXT: []string{"v=spf1 +all"},
		},
		"neutral.maddy.test.": {
			TXT: []string{"v=spf1 ?all"},
		},
		"fail.maddy.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"softfail.maddy.test.": {
			TXT: []string{"v=spf1 ~all"},
		},
		"permerr.maddy.test.": {
			TXT: []string{"v=spf1 something_clever"},
		},
		"temperr.maddy.test.": {
			Err: errors.New("IANA forgot to resign the root zone"),
		},
	})
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no

			check {
				spf {
					enforce_early yes

					none_action reject 551
					neutral_action reject
					fail_action reject 552
					softfail_action reject 553
					permerr_action reject 554
					temperr_action reject 455
				}
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)

	conn.Writeln("MAIL FROM:<testing@pass.maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("RSET")
	conn.ExpectPattern("250 *")

	conn.Writeln("MAIL FROM:<testing@none.maddy.test>")
	conn.ExpectPattern("551 5.7.0 *")

	// Also check the default enhanced code is meaningful.
	conn.Writeln("MAIL FROM:<testing@neutral.maddy.test>")
	conn.ExpectPattern("550 5.7.23 *")

	conn.Writeln("MAIL FROM:<testing@fail.maddy.test>")
	conn.ExpectPattern("552 5.7.0 *")

	conn.Writeln("MAIL FROM:<testing@softfail.maddy.test>")
	conn.ExpectPattern("553 5.7.0 *")

	conn.Writeln("MAIL FROM:<testing@permerr.maddy.test>")
	conn.ExpectPattern("554 5.7.0 *")

	conn.Writeln("MAIL FROM:<testing@temperr.maddy.test>")
	conn.ExpectPattern("455 4.7.0 *")

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestSPF_DMARCDefer(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		"subdomain.maddy-dmarc.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"maddy-dmarc.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"_dmarc.maddy-dmarc.test.": {
			TXT: []string{"v=DMARC1; p=reject; sp=none"},
		},
		"subdomain.maddy-dmarc2.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"maddy-dmarc2.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"_dmarc.maddy-dmarc2.test.": {
			TXT: []string{"v=DMARC1; p=reject"},
		},
		"maddy-no-dmarc.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"maddy-dmarc-lookup-fail.test.": {
			TXT: []string{"v=spf1 -all"},
		},
		"_dmarc.maddy-dmarc-lookup-fail.test.": {
			Err: errors.New("nop"),
		},
	})
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no

			check {
				spf {
					enforce_early no

					none_action ignore
					neutral_action reject
					fail_action reject
					softfail_action reject
					permerr_action reject
					temperr_action reject
				}
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)

	msg := func(fromEnv, fromHdr, bodyRespPattern string) {
		tt.Helper()

		conn.Writeln("MAIL FROM:<" + fromEnv + ">")
		conn.ExpectPattern("250 *")
		conn.Writeln("RCPT TO:<testing@maddy.test>")
		conn.ExpectPattern("250 *")
		conn.Writeln("DATA")
		conn.ExpectPattern("354 *")
		conn.Writeln("From: <" + fromHdr + ">")
		conn.Writeln("")
		conn.Writeln("Heya!")
		conn.Writeln(".")
		conn.ExpectPattern(bodyRespPattern)
	}

	msg("test@subdomain.maddy-dmarc.test", "test@subdomain.maddy-dmarc.test", "550 *")

	// Malformed From domain, DMARC cannot work so use only SPF.
	msg("test@subdomain.maddy-dmarc.test", "", "550 *")

	msg("test@subdomain.maddy-dmarc.test", "maddy-dmarc-lookup-fail.test", "550 *")

	// No actual DMARC check is done but SPF check results are not applied.
	msg("test@maddy-dmarc.test", "test@maddy-dmarc.test", "250 *")
	msg("test@maddy-dmarc2.test", "test@maddy-dmarc2.test", "250 *")

	msg("test@maddy-no-dmarc.test", "test@maddy-no-dmarc.test", "550 *")

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestDNSBLConfig(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		tests.DefaultSourceIPRev + ".dnsbl.test.": {
			A: []string{"127.0.0.127"},
		},
		"sender.test.dnsbl.test.": {
			A: []string{"127.0.0.127"},
		},
	})
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no

			check {
				dnsbl {
					reject_threshold 1

					dnsbl.test {
						client_ipv4
						mailfrom
					}
				}
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)

	conn.Writeln("MAIL FROM:<testing@sender.test>")
	conn.ExpectPattern("554 5.7.0 Client identity is listed in the used DNSBL *")

	conn.Writeln("MAIL FROM:<testing@misc.test>")
	conn.ExpectPattern("554 5.7.0 Client identity is listed in the used DNSBL *")

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestDNSBLConfig2(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(map[string]mockdns.Zone{
		tests.DefaultSourceIPRev + ".dnsbl2.test.": {
			A: []string{"127.0.0.127"},
		},
		"sender.test.dnsbl.test.": {
			A: []string{"127.0.0.127"},
		},
	})
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no

			check {
				dnsbl {
					reject_threshold 1

					dnsbl.test {
						mailfrom
					}
					dnsbl2.test {
						client_ipv4
						score -1
					}
				}
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)

	conn.Writeln("MAIL FROM:<testing@sender.test>")
	conn.ExpectPattern("250 *")

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestCheckAuthorizeSender(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			auth dummy
			defer_sender_reject off

			source example1.org {
				check {
					authorize_sender {
						auth_normalize precis_casefold
						user_to_email static {
							entry "test-user1" "test@example1.org"
							entry "test-user2" "é@example1.org" 
						}
					}
				}
				deliver_to dummy
			}
			source example2.org {
				check {
					authorize_sender {
						auth_normalize precis_casefold
						prepare_email static {
							entry "alias-to-test@example2.org" "test@example2.org"
						}
						user_to_email static {
							entry "test-user1" "test@example2.org"
							entry "test-user2" "test2@example2.org"
						}
					}
				}
				deliver_to dummy
			}

			default_source {
				reject
			}
		}`)
	t.Run(1)
	defer t.Close()

	c := t.Conn("smtp")
	c.SMTPNegotation("client.maddy.test", nil, nil)
	c.SMTPPlainAuth("test-user2", "1", true)
	c.Writeln("MAIL FROM:<test@example1.org>")
	c.ExpectPattern("5*") // rejected - user is not test-user1
	c.Writeln("MAIL FROM:<test3@example1.org>")
	c.ExpectPattern("5*") // rejected - unknown email
	c.Writeln("MAIL FROM:<E\u0301@EXAMPLE1.org> SMTPUTF8")
	c.ExpectPattern("2*") // OK - é@example1.org belongs to test-user2
	c.Close()

	c = t.Conn("smtp")
	c.SMTPNegotation("client.maddy.test", nil, nil)
	c.SMTPPlainAuth("test-user1", "1", true)
	c.Writeln("MAIL FROM:<test2@example2.org>")
	c.ExpectPattern("5*") // rejected - user is not test-user2
	c.Writeln("MAIL FROM:<test@example2.org>")
	c.ExpectPattern("2*") // OK - test@example2.org belongs to test-user
	c.Writeln("MAIL FROM:<alias-to-test@example2.org>")
	c.ExpectPattern("2*") // OK - test@example2.org belongs to test-user
	c.Close()
}

func TestCheckCommand(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			check {
				command {env:TEST_PWD}/testdata/check_command.sh {sender} {
					code 12 reject
				}
			}
			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)

	// Note: Internally, messages are handled using LF line endings, being
	// converted CRLF only when transfered over Internet protocols.
	expectedMsg := "From: <testing@sender.test>\n" +
		"To: <testing@maddy.test>\n" +
		"Subject: Hi there!\n" +
		"\n" +
		"Nice to meet you!\n"
	submitMsg := func(conn *tests.Conn, from string) {
		// Fairly trivial SMTP transaction.
		conn.Writeln("MAIL FROM:<" + from + ">")
		conn.ExpectPattern("250 *")
		conn.Writeln("RCPT TO:<testing@maddy.test>")
		conn.ExpectPattern("250 *")
		conn.Writeln("DATA")
		conn.ExpectPattern("354 *")
		conn.Writeln("From: <testing@sender.test>")
		conn.Writeln("To: <testing@maddy.test>")
		conn.Writeln("Subject: Hi there!")
		conn.Writeln("")
		conn.Writeln("Nice to meet you!")
		conn.Writeln(".")
	}

	t.Subtest("Message dump", func(t *tests.T) {
		conn := conn.Rebind(t)

		submitMsg(conn, "testing@maddy.test")
		conn.ExpectPattern("250 *")

		msgPath := filepath.Join(t.StateDir(), "msg")
		msgContents, err := ioutil.ReadFile(msgPath)
		if err != nil {
			t.Fatal(err)
		}

		if string(msgContents) != expectedMsg {
			t.Log("Wrong message contents received by check script!")
			t.Log("Actual:")
			t.Log(msgContents)
			t.Log("Expected:")
			t.Log(expectedMsg)
		}
	})
	t.Subtest("Message dump + Add header", func(t *tests.T) {
		conn := conn.Rebind(t)

		submitMsg(conn, "testing+addHeader@maddy.test")
		conn.ExpectPattern("250 *")

		msgPath := filepath.Join(t.StateDir(), "msg")
		msgContents, err := ioutil.ReadFile(msgPath)
		if err != nil {
			t.Fatal(err)
		}

		expectedMsg := "X-Added-Header: 1\n" + expectedMsg
		if string(msgContents) != expectedMsg {
			t.Log("Wrong message contents received by check script!")
			t.Log("Actual:")
			t.Log(msgContents)
			t.Log("Expected:")
			t.Log(expectedMsg)
		}
	})
	t.Subtest("Body reject", func(t *tests.T) {
		conn := conn.Rebind(t)

		submitMsg(conn, "testing+reject@maddy.test")
		conn.ExpectPattern("550 *")

		msgPath := filepath.Join(t.StateDir(), "msg")
		msgContents, err := ioutil.ReadFile(msgPath)
		if err != nil {
			t.Fatal(err)
		}

		if string(msgContents) != expectedMsg {
			t.Log("Wrong message contents received by check script!")
			t.Log("Actual:")
			t.Log(msgContents)
			t.Log("Expected:")
			t.Log([]byte(expectedMsg))
		}
	})

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}

func TestHeaderSizeConstraint(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off
			deliver_to dummy
			max_header_size 1K
		}
	`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.SMTPNegotation("localhost", nil, nil)
	conn.Writeln("MAIL FROM:<testsender@maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("RCPT TO:<testing@maddy.test>")
	conn.ExpectPattern("250 *")
	conn.Writeln("DATA")
	conn.ExpectPattern("354 *")
	conn.Writeln("From: <testing@sender.test>")
	conn.Writeln("To: <testing@maddy.test>")
	conn.Writeln("Subject: " + strings.Repeat("A", 2*1024))
	conn.Writeln("")
	conn.Writeln("Hi")
	conn.Writeln(".")

	conn.ExpectPattern("552 5.3.4 Message header size exceeds limit *")

	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}
