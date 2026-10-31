//go:build integration
// +build integration

/*
Maddy Mail Server - Composable all-in-one mail server.
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
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/foxcpp/maddy/tests"
)

var FetchmailExecutable string

func init() {
	flag.StringVar(&FetchmailExecutable, "integration.fetchmail", "fetchmail", "path to fetchmail executable for interop tests")
}

func TestFetchmailIMAPDownload(tt *testing.T) {
	tt.Parallel()

	fetchmailExec, err := exec.LookPath(FetchmailExecutable)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			tt.Skip("No fetchmail executable found, skipping interop test")
		}
		tt.Fatal(err)
	}

	t := tests.NewT(tt)
	t.DNS(nil)
	imapPort := t.Port("imap")
	t.Config(`
		storage.imapsql local_mailboxes {
			driver sqlite3
			dsn imapsql.db
		}

		imap tcp://127.0.0.1:{env:TEST_PORT_imap} {
			tls off

			auth dummy
			storage &local_mailboxes
		}
	`)
	t.Run(1)
	defer t.Close()

	const username = "fetchmail-test@maddy.test"
	messages := []struct {
		id   string
		body string
	}{
		{id: "<fetchmail-test-1@maddy.test>", body: "fetchmail unique body one"},
		{id: "<fetchmail-test-2@maddy.test>", body: "fetchmail unique body two"},
		{id: "<fetchmail-test-3@maddy.test>", body: "fetchmail unique body three"},
	}

	imapConn := t.Conn("imap")
	defer imapConn.Close()
	imapConn.ExpectPattern(`\* OK *`)
	imapConn.Writeln(". LOGIN " + username + " 1234")
	imapConn.ExpectPattern(". OK *")
	imapConn.Writeln(". SELECT INBOX")
	for i := 0; i < 6; i++ {
		imapConn.ExpectPattern(`\* *`)
	}
	imapConn.ExpectPattern(". OK *")

	for i, msg := range messages {
		raw := fmt.Sprintf("From: sender@maddy.test\r\nTo: %s\r\nMessage-ID: %s\r\nSubject: fetchmail test %d\r\nDate: Fri, 30 Oct 2026 12:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", username, msg.id, i+1, msg.body)
		imapConn.Writeln(fmt.Sprintf(". APPEND INBOX {%d}", len(raw)))
		imapConn.ExpectPattern("+ *")
		imapConn.Write(raw + "\r\n")
		imapConn.ExpectPattern(fmt.Sprintf("* %d EXISTS", i+1))
		imapConn.ExpectPattern(fmt.Sprintf("* %d RECENT", i+1))
		imapConn.ExpectPattern(". OK *")
	}

	tempDir := t.TempDir()
	rcPath := filepath.Join(tempDir, "fetchmailrc")
	bsmtpPath := filepath.Join(tempDir, "messages.bsmtp")
	idPath := filepath.Join(tempDir, "fetchmail.ids")
	rc := fmt.Sprintf(`poll 127.0.0.1 proto IMAP service %d
    user "%s" there with password "1234" is "local@maddy.test" here
    folder INBOX
    keep
    sslproto ""
`, imapPort, username)
	if err := os.WriteFile(rcPath, []byte(rc), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(fetchmailExec,
		"--fetchmailrc", rcPath,
		"--idfile", idPath,
		"--bsmtp", bsmtpPath,
		"--nodetach", "--nosyslog", "-v",
	)
	cmd.Env = append(os.Environ(), "HOME="+tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fetchmail failed: %v\n%s", err, output)
	}
	t.Logf("fetchmail output:\n%s", output)

	bsmtp, err := os.ReadFile(bsmtpPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(bsmtp, []byte("DATA\r\n")); got != len(messages) {
		t.Fatalf("fetchmail delivered %d messages; want %d\nBSMTP output:\n%s", got, len(messages), bsmtp)
	}
	for _, msg := range messages {
		for _, content := range []string{msg.id, msg.body} {
			if got := bytes.Count(bsmtp, []byte(content)); got != 1 {
				t.Errorf("fetchmail BSMTP output contains %q %d times; want exactly once", content, got)
			}
		}
	}
}
