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
	"strconv"
	"strings"
	"testing"

	"github.com/foxcpp/maddy/internal/testutils"
	"github.com/foxcpp/maddy/tests"
)

func TestLMTPServer_Is_Actually_LMTP(tt *testing.T) {
	tt.Parallel()

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("lmtp")
	t.Config(`
		lmtp tcp://127.0.0.1:{env:TEST_PORT_lmtp} {
			hostname mx.maddy.test
			tls off
			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	c := t.Conn("lmtp")
	defer c.Close()

	c.Writeln("LHLO client.maddy.test")
	c.ExpectPattern("220 *")
capsloop:
	for {
		line, err := c.Readln()
		if err != nil {
			t.Fatal("I/O error:", err)
		}
		switch {
		case strings.HasPrefix(line, "250-"):
		case strings.HasPrefix(line, "250 "):
			break capsloop
		default:
			t.Fatal("Unexpected deply:", line)
		}
	}

	c.Writeln("MAIL FROM:<from@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to1@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to2@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("DATA")
	c.ExpectPattern("354 *")
	c.Writeln("From: <from@maddy.test>")
	c.Writeln("To: <to@maddy.test>")
	c.Writeln("Subject: Hello!")
	c.Writeln("")
	c.Writeln("Hello!")
	c.Writeln(".")
	c.ExpectPattern("250 2.0.0 <to1@maddy.test> OK: queued")
	c.ExpectPattern("250 2.0.0 <to2@maddy.test> OK: queued")
	c.Writeln("QUIT")
	c.ExpectPattern("221 *")
}

func TestLMTPClient_Is_Actually_LMTP(tt *testing.T) {
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	tgtPort := t.Port("target")
	t.Config(`
		hostname mx.maddy.test
		tls off
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			deliver_to lmtp tcp://127.0.0.1:{env:TEST_PORT_target}
		}`)
	t.Run(1)
	defer t.Close()

	be, s := testutils.SMTPServer(tt, "127.0.0.1:"+strconv.Itoa(int(tgtPort)))
	s.LMTP = true
	be.LMTPDataErr = []error{nil, nil}
	defer s.Close()

	c := t.Conn("smtp")
	defer c.Close()
	c.SMTPNegotation("client.maddy.test", nil, nil)
	c.Writeln("MAIL FROM:<from@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to1@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to2@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("DATA")
	c.ExpectPattern("354 *")
	c.Writeln("From: <from@maddy.test>")
	c.Writeln("To: <to@maddy.test>")
	c.Writeln("Subject: Hello!")
	c.Writeln("")
	c.Writeln("Hello!")
	c.Writeln(".")
	c.ExpectPattern("250 2.0.0 OK: queued")
	c.Writeln("QUIT")
	c.ExpectPattern("221 *")

	if be.SessionCounter != 1 {
		t.Fatal("No actual connection made?", be.SessionCounter)
	}
}

func TestLMTPClient_Issue308(tt *testing.T) {
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	tgtPort := t.Port("target")
	t.Config(`
		hostname mx.maddy.test
		tls off

		target.lmtp local_mailboxes {
			targets tcp://127.0.0.1:{env:TEST_PORT_target}
		}

		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			deliver_to &local_mailboxes
		}`)
	t.Run(1)
	defer t.Close()

	be, s := testutils.SMTPServer(tt, "127.0.0.1:"+strconv.Itoa(int(tgtPort)))
	s.LMTP = true
	be.LMTPDataErr = []error{nil, nil}
	defer s.Close()

	c := t.Conn("smtp")
	defer c.Close()
	c.SMTPNegotation("client.maddy.test", nil, nil)
	c.Writeln("MAIL FROM:<from@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to1@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("RCPT TO:<to2@maddy.test>")
	c.ExpectPattern("250 *")
	c.Writeln("DATA")
	c.ExpectPattern("354 *")
	c.Writeln("From: <from@maddy.test>")
	c.Writeln("To: <to@maddy.test>")
	c.Writeln("Subject: Hello!")
	c.Writeln("")
	c.Writeln("Hello!")
	c.Writeln(".")
	c.ExpectPattern("250 2.0.0 OK: queued")
	c.Writeln("QUIT")
	c.ExpectPattern("221 *")

	if be.SessionCounter != 1 {
		t.Fatal("No actual connection made?", be.SessionCounter)
	}
}
