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

package smtpconn

import (
	"context"
	"errors"
	"flag"
	"math/rand"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/internal/testutils"
)

var testPort string

func TestMain(m *testing.M) {
	remoteSmtpPort := flag.String("test.smtpport", "random", "(maddy) SMTP port to use for connections in tests")
	flag.Parse()

	if *remoteSmtpPort == "random" {
		*remoteSmtpPort = strconv.Itoa(rand.Intn(65536-10000) + 10000)
	}

	testPort = *remoteSmtpPort
	os.Exit(m.Run())
}

// TestConnectRefused checks that a failed dial is reported as a temporary
// "Network I/O error", so the sender retries instead of bouncing the message.
func TestConnectRefused(t *testing.T) {
	// Get a local port with nothing listening on it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	endp := config.Endpoint{Scheme: "tcp", Host: "127.0.0.1", Port: port}

	connect := map[string]func(*C) error{
		"Connect": func(c *C) error {
			_, err := c.Connect(context.Background(), endp, false, nil)
			return err
		},
		"ConnectLMTP": func(c *C) error {
			_, err := c.ConnectLMTP(context.Background(), endp, false, nil)
			return err
		},
	}
	for name, fn := range connect {
		t.Run(name, func(t *testing.T) {
			c := New()
			c.Log = testutils.Logger(t, "smtpconn")

			err := fn(c)
			if err == nil {
				t.Fatal("expected an error")
			}
			var smtpErr *exterrors.SMTPError
			if !errors.As(err, &smtpErr) {
				t.Fatalf("expected *exterrors.SMTPError, got %T: %v", err, err)
			}
			if smtpErr.Code != 450 || smtpErr.EnhancedCode != (exterrors.EnhancedCode{4, 4, 2}) {
				t.Errorf("expected 450 4.4.2, got %d %v", smtpErr.Code, smtpErr.EnhancedCode)
			}
			if !exterrors.IsTemporary(err) {
				t.Error("expected a temporary error")
			}
		})
	}
}
