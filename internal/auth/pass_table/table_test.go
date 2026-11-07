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

package pass_table

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/auth"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestAuth_AuthPlain(t *testing.T) {
	addSHA256()

	mod, err := New(container.New(), "auth.pass_table", "local_auth")
	if err != nil {
		t.Fatal(err)
	}
	err = mod.Configure([]string{"dummy"}, config.NewMap(nil, config.Node{
		Children: []config.Node{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	a := mod.(*Auth)
	a.table = testutils.Table{
		M: map[string]string{
			"postmaster@example.org": "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw=",
			"postmaster@other.org":   "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw=",
			"foxcpp":                 "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw=",
			"not-foxcpp":             "bcrypt:$2y$10$4tEJtJ6dApmhETg8tJ4WHOeMtmYXQwmHDKIyfg09Bw1F/smhLjlaa",
			"not-foxcpp-2":           "argon2:1:8:1:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q==",
		},
	}

	check := func(user, pass string, ok bool) {
		t.Helper()

		err := a.AuthPlain(user, pass)
		if (err == nil) != ok {
			t.Errorf("ok=%v, err: %v", ok, err)
		}
	}

	check("foxcpp", "password", true)
	check("foxcpp", "different-password", false)
	check("not-foxcpp", "password", true)
	check("not-foxcpp", "different-password", false)
	check("not-foxcpp-2", "password", true)
	check("postmaster@example.org", "password", true)
	check("POSTMASTER@EXAMPLE.ORG", "password", true)
	check("postmaster@other.org", "password", true)

	for _, username := range []string{"postmaster@example.or", "POSTMASTER@EXAMPLE.OR"} {
		err := a.AuthPlain(username, "secret-password")
		if !errors.Is(err, module.ErrUnknownCredentials) {
			t.Fatalf("expected unknown credentials, got %v", err)
		}
		fields := exterrors.Fields(err)
		for key, want := range map[string]string{
			"reason":          "username not found in credentials table",
			"module":          "auth.pass_table",
			"instance":        "local_auth",
			"mapped_username": "postmaster@example.or",
		} {
			if fields[key] != want {
				t.Errorf("%s = %v, want %q", key, fields[key], want)
			}
		}
		if strings.Contains(fmt.Sprint(err, fields), "secret-password") {
			t.Error("password leaked in error")
		}
	}

	err = a.AuthPlain("postmaster@example.org", "wrong-password")
	if err == nil || exterrors.Fields(err)["reason"] != nil {
		t.Fatalf("wrong password classified as missing username: %v", err)
	}
	if strings.Contains(err.Error(), "wrong-password") || strings.Contains(err.Error(), a.table.(testutils.Table).M["postmaster@example.org"]) {
		t.Error("credentials leaked in error")
	}

	lookupErr := errors.New("table lookup failed")
	for _, username := range []string{"postmaster@example.or", "postmaster@example.org"} {
		table := a.table.(testutils.Table)
		table.Err = lookupErr
		a.table = table
		err := a.AuthPlain(username, "password")
		if !errors.Is(err, lookupErr) || exterrors.Fields(err)["reason"] != nil {
			t.Errorf("lookup error for %s = %v, fields: %v", username, err, exterrors.Fields(err))
		}
	}
}

func TestAuth_CreateSASL(t *testing.T) {
	addSHA256()
	const hash = "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw="
	provider := &Auth{
		modName: "auth.pass_table", instName: "local_auth",
		table: testutils.Table{M: map[string]string{"postmaster@example.org": hash}},
	}
	for _, mech := range []string{"PLAIN", "LOGIN"} {
		t.Run(mech, func(t *testing.T) {
			for _, withErrorMap := range []bool{false, true} {
				var output string
				a := auth.SASLAuth{
					Log: &log.Logger{Out: log.FuncOutput(func(_ time.Time, debug bool, msg string) {
						if !debug {
							output += msg
						}
					}, nil)},
					EnableLogin: true,
					Plain:       []module.PlainAuth{provider},
				}
				mapCalls := 0
				if withErrorMap {
					a.ErrorMap = func(err error) error {
						mapCalls++
						if err != auth.ErrInvalidAuthCred || len(exterrors.Fields(err)) != 0 {
							t.Errorf("ErrorMap received detailed error: %v", err)
						}
						return err
					}
				}
				for _, username := range []string{"POSTMASTER@EXAMPLE.OR", "postmaster@example.org"} {
					output = ""
					remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 12345}
					srv := a.CreateSASL(mech, remote, func(string, auth.ContextData) error {
						t.Fatal("unexpected authentication success")
						return nil
					})
					var err error
					if mech == "PLAIN" {
						_, _, err = srv.Next([]byte("\x00" + username + "\x00secret-password"))
					} else {
						if _, _, err := srv.Next(nil); err != nil {
							t.Fatal(err)
						}
						if _, _, err := srv.Next([]byte(username)); err != nil {
							t.Fatal(err)
						}
						_, _, err = srv.Next([]byte("secret-password"))
					}
					if err != auth.ErrInvalidAuthCred {
						t.Fatalf("client error = %v, want generic invalid credentials", err)
					}
					_, raw, ok := strings.Cut(output, "\t")
					if !ok {
						t.Fatalf("missing structured log: %q", output)
					}
					var fields map[string]interface{}
					if err := json.Unmarshal([]byte(raw), &fields); err != nil {
						t.Fatal(err)
					}
					if fields["username"] != username || fields["src_ip"] != remote.String() {
						t.Errorf("missing authentication context: %v", fields)
					}
					if username == "POSTMASTER@EXAMPLE.OR" {
						for key, want := range map[string]string{
							"reason":          "username not found in credentials table",
							"module":          "auth.pass_table",
							"instance":        "local_auth",
							"mapped_username": "postmaster@example.or",
						} {
							if fields[key] != want {
								t.Errorf("log %s = %v, want %q", key, fields[key], want)
							}
						}
					} else if fields["reason"] == "username not found in credentials table" {
						t.Error("wrong password logged as missing username")
					}
					if strings.Contains(output, "secret-password") || strings.Contains(output, hash) {
						t.Error("credentials leaked in log")
					}
				}
				if withErrorMap && mapCalls != 2 {
					t.Errorf("ErrorMap calls = %d, want 2", mapCalls)
				}
			}
		})
	}

	lastErr := errors.New("last provider lookup failed")
	last := &Auth{table: testutils.Table{M: map[string]string{"other": hash}}}
	a := auth.SASLAuth{Log: &log.NopLogger, Plain: []module.PlainAuth{provider, last}}
	if err := a.AuthPlain("other", "password"); err != nil {
		t.Fatalf("provider fallback failed: %v", err)
	}
	last.table = testutils.Table{Err: lastErr}
	if err := a.AuthPlain("other", "password"); !errors.Is(err, lastErr) || exterrors.Fields(err)["reason"] != nil {
		t.Fatalf("last provider error lost: %v", err)
	}
}
