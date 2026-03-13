//go:build unix && integration

// Can't reload on Windows, yet

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2026 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

	sqliteprovider "github.com/foxcpp/maddy/internal/sqlite"
	"github.com/foxcpp/maddy/tests"
)

func TestSmtpPipelineSwitch(tt *testing.T) {
	if !sqliteprovider.IsTranspiled {
		tt.Skip("Test is unstable with original SQLite")
	}

	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			reject
		}
	`)
	t.Run(1)
	defer t.Close()

	conn1 := t.Conn("smtp")
	defer conn1.Close()
	conn1.SMTPNegotation("localhost", nil, nil)
	conn1.Writeln("MAIL FROM:<sender@maddy.test>")
	conn1.ExpectPattern("2*")
	conn1.Writeln("RCPT TO:<testusr@maddy.test>")
	conn1.ExpectPattern("5*") // REJECTED
	conn1.Writeln("RSET")
	conn1.ExpectPattern("2*")

	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname maddy.test
			tls off

			deliver_to dummy
		}
	`)

	conn2 := t.Conn("smtp")
	defer conn2.Close()
	conn2.SMTPNegotation("localhost", nil, nil)
	conn2.Writeln("MAIL FROM:<sender@maddy.test>")
	conn2.ExpectPattern("2*")
	conn2.Writeln("RCPT TO:<testusr@maddy.test>")
	conn2.ExpectPattern("2*")
	conn2.Writeln("DATA")
	conn2.ExpectPattern("354 *")
	conn2.Writeln("From: <sender@maddy.test>")
	conn2.Writeln("To: <testusr@maddy.test>")
	conn2.Writeln("Subject: Hi!")
	conn2.Writeln("")
	conn2.Writeln("Hi!")
	conn2.Writeln(".")
	conn2.ExpectPattern("2*") // DISCARDED

	conn1.Writeln("MAIL FROM:<sender@maddy.test>")
	conn1.ExpectPattern("2*")
	conn1.Writeln("RCPT TO:<testusr@maddy.test>")
	conn1.ExpectPattern("5*") // Still REJECTED (running on old server).
	conn1.Writeln("RSET")
	conn1.ExpectPattern("2*")
}

func TestImapStorageSwitch(tt *testing.T) {
	if !sqliteprovider.IsTranspiled {
		tt.Skip("Test is unstable with original SQLite")
	}

	tt.Parallel()
	t := tests.NewT(tt)

	t.DNS(nil)
	t.Port("smtp")
	t.Port("imap")
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
	t.Run(1)
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

	conn1 := t.Conn("smtp")
	defer conn1.Close()
	conn1.SMTPNegotation("localhost", nil, nil)
	conn1.Writeln("MAIL FROM:<sender@maddy.test>")
	conn1.ExpectPattern("2*")
	conn1.Writeln("RCPT TO:<testusr@maddy.test>")
	conn1.ExpectPattern("2*")
	conn1.Writeln("DATA")
	conn1.ExpectPattern("354 *")
	conn1.Writeln("From: <sender@maddy.test>")
	conn1.Writeln("To: <testusr@maddy.test>")
	conn1.Writeln("Subject: Store 1")
	conn1.Writeln("")
	conn1.Writeln("Hi!")
	conn1.Writeln(".")
	conn1.ExpectPattern("2*") // Goes to storage 1

	t.Config(`
		storage.imapsql test_store {
			driver sqlite3
			dsn imapsql2.db
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

	imapConn2 := t.Conn("imap")
	defer imapConn2.Close()
	imapConn2.ExpectPattern(`\* OK *`)
	imapConn2.Writeln(". LOGIN testusr2@maddy.test 1234")
	imapConn2.ExpectPattern(". OK *")

	time.Sleep(500 * time.Millisecond)

	conn2 := t.Conn("smtp")
	defer conn2.Close()
	conn2.SMTPNegotation("localhost", nil, nil)
	conn2.Writeln("MAIL FROM:<sender@maddy.test>")
	conn2.ExpectPattern("2*")
	conn2.Writeln("RCPT TO:<testusr2@maddy.test>")
	conn2.ExpectPattern("2*")
	conn2.Writeln("DATA")
	conn2.ExpectPattern("354 *")
	conn2.Writeln("From: <sender@maddy.test>")
	conn2.Writeln("To: <testusr2@maddy.test>")
	conn2.Writeln("Subject: Store 2")
	conn2.Writeln("")
	conn2.Writeln("Hi!")
	conn2.Writeln(".")
	conn2.ExpectPattern("2*") // Goes to storage 2

	imapConn.Writeln(". NOOP")
	imapConn.ExpectPattern(`\* 1 EXISTS`)
	imapConn.ExpectPattern(`\* 1 RECENT`)
	imapConn.ExpectPattern(". OK *")

	// Old connection sees message in store 1.
	imapConn.Writeln(". FETCH 1 (BODY.PEEK[])")
	imapConn.ExpectPattern(`\* 1 FETCH (BODY\[\] {*}*`)
	imapConn.Expect(`Delivered-To: testusr@maddy.test`)
	imapConn.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn.ExpectPattern(` *`)
	imapConn.Expect("From: <sender@maddy.test>")
	imapConn.Expect("To: <testusr@maddy.test>")
	imapConn.Expect("Subject: Store 1")
	imapConn.Expect("")
	imapConn.Expect("Hi!")
	imapConn.Expect(")")
	imapConn.ExpectPattern(`. OK *`)

	// New connection sees message in store 2.
	imapConn2.Writeln(". SELECT INBOX")
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`\* *`)
	imapConn2.ExpectPattern(`. OK *`)
	imapConn2.Writeln(". FETCH 1 (BODY.PEEK[])")
	imapConn2.ExpectPattern(`\* 1 FETCH (BODY\[\] {*}*`)
	imapConn2.Expect(`Delivered-To: testusr2@maddy.test`)
	imapConn2.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn2.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn2.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn2.ExpectPattern(` *`)
	imapConn2.Expect("From: <sender@maddy.test>")
	imapConn2.Expect("To: <testusr2@maddy.test>")
	imapConn2.Expect("Subject: Store 2")
	imapConn2.Expect("")
	imapConn2.Expect("Hi!")
	imapConn2.Expect(")")
	imapConn2.ExpectPattern(`. OK *`)

}
