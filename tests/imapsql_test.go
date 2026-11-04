//go:build integration

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
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/tests"
)

type forwardingLMTP struct{ smtpAddr string }
type forwardingLMTPSession struct{ smtpAddr string }

func (b forwardingLMTP) NewSession(*smtp.Conn) (smtp.Session, error) {
	return &forwardingLMTPSession{smtpAddr: b.smtpAddr}, nil
}
func (*forwardingLMTPSession) Reset()                               {}
func (*forwardingLMTPSession) Logout() error                        { return nil }
func (*forwardingLMTPSession) Mail(string, *smtp.MailOptions) error { return nil }
func (*forwardingLMTPSession) Rcpt(string, *smtp.RcptOptions) error { return nil }
func (s *forwardingLMTPSession) Data(r io.Reader) error {
	client, err := smtp.Dial(s.smtpAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Mail("forwarder@maddy.test", nil); err != nil {
		return err
	}
	if err := client.Rcpt("testusr@maddy.test", nil); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Close()
}

func TestImapsqlSynchronousLMTPForwarding(tt *testing.T) {
	// This test will trigger multiple parallel deliveries to the same
	// mailbox in storage.imapsql. We check that all messages
	// are successfully delivered without temporary failures.

	t := tests.NewT(tt)
	t.DNS(nil)

	smtpPort := t.Port("smtp")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Env("TEST_LMTP_ADDR=" + listener.Addr().String())
	server := smtp.NewServer(forwardingLMTP{smtpAddr: "127.0.0.1:" + strconv.Itoa(int(smtpPort))})
	server.LMTP = true
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	t.Port("imap")
	t.Config(`
		hostname maddy.test

		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql.db
			sqlite3_busy_timeout 100
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off
			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			tls off
			destination list@maddy.test {
				deliver_to lmtp tcp://{env:TEST_LMTP_ADDR}
			}
			destination maddy.test {
				deliver_to &test_store
			}
			default_destination { reject }
		}
	`)
	t.Run(2)

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN testusr@maddy.test 1234")
	imapConn.ExpectPattern(". OK *")

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()

	smtpConn.SMTPNegotation("localhost", nil, nil)
	for range 10 {
		smtpConn.Writeln("MAIL FROM:<sender@maddy.test>")
		smtpConn.ExpectPattern("250 *")
		for _, rcpt := range []string{"testusr@maddy.test", "list@maddy.test"} {
			smtpConn.Writeln("RCPT TO:<" + rcpt + ">")
			smtpConn.ExpectPattern("250 *")
		}
		smtpConn.Writeln("DATA")
		smtpConn.ExpectPattern("354 *")
		smtpConn.Writeln("From: <sender@maddy.test>")
		smtpConn.Writeln("Subject: forwarded message")
		smtpConn.Writeln("")
		smtpConn.Writeln("hello")
		smtpConn.Writeln(".")
		smtpConn.ExpectPattern("250 *")
	}

	imapConn.Writeln(". STATUS INBOX (MESSAGES)")
	imapConn.Expect(`* STATUS INBOX (MESSAGES 20)`)
	imapConn.ExpectPattern(". OK *")
}

// Smoke test to ensure message delivery is handled correctly.

func TestImapsqlDelivery(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to &test_store
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN testusr@maddy.test 1234")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". SELECT INBOX")
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`. OK *`)

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	smtpConn.Writeln("MAIL FROM:<sender@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("RCPT TO:<testusr@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("DATA")
	smtpConn.ExpectPattern("354 *")
	smtpConn.Writeln("From: <sender@maddy.test>")
	smtpConn.Writeln("To: <testusr@maddy.test>")
	smtpConn.Writeln("Subject: Hi!")
	smtpConn.Writeln("")
	smtpConn.Writeln("Hi!")
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

	time.Sleep(500 * time.Millisecond)

	imapConn.Writeln(". NOOP")
	imapConn.ExpectPattern(`\* 1 EXISTS`)
	imapConn.ExpectPattern(`\* 1 RECENT`)
	imapConn.ExpectPattern(". OK *")

	imapConn.Writeln(". FETCH 1 (BODY.PEEK[])")
	imapConn.ExpectPattern(`\* 1 FETCH (BODY\[\] {*}*`)
	imapConn.Expect(`Delivered-To: testusr@maddy.test`)
	imapConn.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn.ExpectPattern(` *`)
	imapConn.Expect("From: <sender@maddy.test>")
	imapConn.Expect("To: <testusr@maddy.test>")
	imapConn.Expect("Subject: Hi!")
	imapConn.Expect("")
	imapConn.Expect("Hi!")
	imapConn.Expect(")")
	imapConn.ExpectPattern(`. OK *`)
}

// TestImapsqlRerouteAndDirectDelivery covers issue #786: the same SMTP
// transaction reaches the SQLite store both directly and through rerouting.
func TestImapsqlRerouteAndDirectDelivery(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		hostname maddy.test
		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql.db
		}
		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off
			auth dummy
			storage &test_store
		}
		msgpipeline local_routing {
			destination list@maddy.test {
				modify {
					replace_rcpt static {
						entry list@maddy.test testusr@maddy.test
					}
				}
				reroute {
					destination maddy.test {
						deliver_to &test_store
					}
					default_destination {
						reject
					}
				}
			}
			destination maddy.test {
				deliver_to &test_store
			}
			default_destination {
				reject
			}
		}
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off
			deliver_to &local_routing
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN testusr@maddy.test 1234")
	imapConn.ExpectPattern(". OK *")

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	for _, recipients := range [][]string{
		{"list@maddy.test", "testusr@maddy.test"},
		{"testusr@maddy.test"},
	} {
		smtpConn.Writeln("MAIL FROM:<sender@maddy.test>")
		smtpConn.ExpectPattern("250 *")
		for _, recipient := range recipients {
			smtpConn.Writeln("RCPT TO:<" + recipient + ">")
			smtpConn.ExpectPattern("250 *")
		}
		smtpConn.Writeln("DATA")
		smtpConn.ExpectPattern("354 *")
		smtpConn.Writeln("From: <sender@maddy.test>")
		smtpConn.Writeln("To: <testusr@maddy.test>, <list@maddy.test>")
		smtpConn.Writeln("Subject: Issue 786")
		smtpConn.Writeln("")
		smtpConn.Writeln("Test message")
		smtpConn.Writeln(".")
		smtpConn.ExpectPattern("250 *")
	}

	imapConn.Writeln(". STATUS INBOX (MESSAGES)")
	// Depending on recipient deduplication, the first message can be stored
	// once or twice; the subsequent message must also be present.
	imapConn.ExpectPattern(`\* STATUS INBOX (MESSAGES [23])`)
	imapConn.ExpectPattern(". OK *")
}

func TestImapsqlDeliveryMap(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		storage.imapsql test_store {
			delivery_map email_localpart
			auth_normalize precis

			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to &test_store
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN testusr 1234")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". SELECT INBOX")
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`. OK *`)

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	smtpConn.Writeln("MAIL FROM:<sender@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("RCPT TO:<testusr@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("DATA")
	smtpConn.ExpectPattern("354 *")
	smtpConn.Writeln("From: <sender@maddy.test>")
	smtpConn.Writeln("To: <testusr@maddy.test>")
	smtpConn.Writeln("Subject: Hi!")
	smtpConn.Writeln("")
	smtpConn.Writeln("Hi!")
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

	time.Sleep(500 * time.Millisecond)

	imapConn.Writeln(". NOOP")
	imapConn.ExpectPattern(`\* 1 EXISTS`)
	imapConn.ExpectPattern(`\* 1 RECENT`)
	imapConn.ExpectPattern(". OK *")
}

func TestImapsqlAuthMap(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("imap")
	t.Port("smtp")
	t.Config(`
		storage.imapsql test_store {
			auth_map regexp "(.*)" "$1@maddy.test"
			auth_normalize precis

			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth dummy
			storage &test_store
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to &test_store
		}
	`)
	t.Run(2)
	defer t.Close()

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN testusr 1234")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". SELECT INBOX")
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`\* *`)
	imapConn.ExpectPattern(`. OK *`)

	smtpConn := t.Conn("smtp")
	defer smtpConn.Close()
	smtpConn.SMTPNegotation("localhost", nil, nil)
	smtpConn.Writeln("MAIL FROM:<sender@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("RCPT TO:<testusr@maddy.test>")
	smtpConn.ExpectPattern("2*")
	smtpConn.Writeln("DATA")
	smtpConn.ExpectPattern("354 *")
	smtpConn.Writeln("From: <sender@maddy.test>")
	smtpConn.Writeln("To: <testusr@maddy.test>")
	smtpConn.Writeln("Subject: Hi!")
	smtpConn.Writeln("")
	smtpConn.Writeln("Hi!")
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

	time.Sleep(500 * time.Millisecond)

	imapConn.Writeln(". NOOP")
	imapConn.ExpectPattern(`\* 1 EXISTS`)
	imapConn.ExpectPattern(`\* 1 RECENT`)
	imapConn.ExpectPattern(". OK *")
}
