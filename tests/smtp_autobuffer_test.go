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
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/maddy/tests"
)

func TestSMTPEndpoint_LargeMessage(tt *testing.T) {
	// Send 1.44 MiB message to verify it being handled correctly
	// everywhere.
	// (Issue 389)

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
	for i := 0; i < 3000; i++ {
		smtpConn.Writeln(strings.Repeat("A", 500))
	}
	// 3000*502 ~ 1.44 MiB not including header side
	smtpConn.Writeln(".")

	time.Sleep(500 * time.Millisecond)

	imapConn.Writeln(". NOOP")
	imapConn.ExpectPattern(`\* 1 EXISTS`)
	imapConn.ExpectPattern(`\* 1 RECENT`)
	imapConn.ExpectPattern(". OK *")

	imapConn.Writeln(". FETCH 1 (BODY.PEEK[])")
	imapConn.ExpectPattern(`\* 1 FETCH (BODY\[\] {1506312}*`)
	imapConn.Expect(`Delivered-To: testusr@maddy.test`)
	imapConn.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn.ExpectPattern(` *`)
	imapConn.Expect("From: <sender@maddy.test>")
	imapConn.Expect("To: <testusr@maddy.test>")
	imapConn.Expect("Subject: Hi!")
	imapConn.Expect("")
	for i := 0; i < 3000; i++ {
		imapConn.Expect(strings.Repeat("A", 500))
	}
	imapConn.Expect(")")
	imapConn.ExpectPattern(`. OK *`)
}

func TestSMTPEndpoint_FileBuffer(tt *testing.T) {
	run := func(tt *testing.T, bufferOpt string) {
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
			buffer ` + bufferOpt + `

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
		smtpConn.Writeln("AAAAABBBBBB")
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
		imapConn.Expect("AAAAABBBBBB")
		imapConn.Expect(")")
		imapConn.ExpectPattern(`. OK *`)
	}

	tt.Run("ram", func(tt *testing.T) { run(tt, "ram") })
	tt.Run("fs", func(tt *testing.T) { run(tt, "fs") })
}

func TestSMTPEndpoint_Autobuffer(tt *testing.T) {
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
			buffer auto 5b

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
	smtpConn.Writeln("AAAAABBBBBB")
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

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
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

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
	smtpConn.Writeln("AAA")
	smtpConn.Writeln(".")
	smtpConn.ExpectPattern("2*")

	time.Sleep(500 * time.Millisecond)

	imapConn.Writeln(". NOOP")
	// This will break with go-imap v2 upgrade merging updates.
	imapConn.ExpectPattern(`\* 3 EXISTS`)
	imapConn.ExpectPattern(`\* 3 RECENT`)
	imapConn.ExpectPattern(". OK *")

	imapConn.Writeln(". FETCH 1:3 (BODY.PEEK[])")
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
	imapConn.Expect("AAAAABBBBBB")
	imapConn.Expect(")")
	imapConn.ExpectPattern(`\* 2 FETCH (BODY\[\] {*}*`)
	imapConn.Expect(`Delivered-To: testusr@maddy.test`)
	imapConn.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn.ExpectPattern(` *`)
	imapConn.Expect("From: <sender@maddy.test>")
	imapConn.Expect("To: <testusr@maddy.test>")
	imapConn.Expect("Subject: Hi!")
	imapConn.Expect("")
	imapConn.Expect(")")
	imapConn.ExpectPattern(`\* 3 FETCH (BODY\[\] {*}*`)
	imapConn.Expect(`Delivered-To: testusr@maddy.test`)
	imapConn.Expect(`Return-Path: <sender@maddy.test>`)
	imapConn.ExpectPattern(`Received: from localhost (client.maddy.test \[` + tests.DefaultSourceIP.String() + `\]) by maddy.test`)
	imapConn.ExpectPattern(` (envelope-sender <sender@maddy.test>) with ESMTP id *; *`)
	imapConn.ExpectPattern(` *`)
	imapConn.Expect("From: <sender@maddy.test>")
	imapConn.Expect("To: <testusr@maddy.test>")
	imapConn.Expect("Subject: Hi!")
	imapConn.Expect("")
	imapConn.Expect("AAA")
	imapConn.Expect(")")
	imapConn.ExpectPattern(`. OK *`)
}
