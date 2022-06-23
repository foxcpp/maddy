//go:build integration && cgo && !nosqlite3
// +build integration,cgo,!nosqlite3

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
	"testing"
	"time"

	"github.com/foxcpp/maddy/tests"
)

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
