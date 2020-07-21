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

package auth

import (
	"errors"
	"net"
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

type mockAuth struct {
	db map[string]bool
}

func (m mockAuth) AuthPlain(username, _ string) error {
	ok := m.db[username]
	if !ok {
		return errors.New("invalid creds")
	}
	return nil
}

func TestCreateSASL(t *testing.T) {
	a := SASLAuth{
		Log: testutils.Logger(t, "saslauth"),
		Plain: []module.PlainAuth{
			&mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	t.Run("XWHATEVER", func(t *testing.T) {
		srv := a.CreateSASL("XWHATEVER", &net.TCPAddr{}, func(string) error { return nil })
		_, _, err := srv.Next([]byte(""))
		if err == nil {
			t.Error("No error for XWHATEVER use")
		}
	})

	t.Run("PLAIN", func(t *testing.T) {
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(id string) error {
			if id != "user1" {
				t.Fatal("Wrong auth. identities passed to callback:", id)
			}
			return nil
		})

		_, _, err := srv.Next([]byte("\x00user1\x00aa"))
		if err != nil {
			t.Error("Unexpected error:", err)
		}
	})

	t.Run("PLAIN with authorization identity", func(t *testing.T) {
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(id string) error {
			if id != "user1a" {
				t.Fatal("Wrong authorization identity passed:", id)
			}
			return nil
		})

		_, _, err := srv.Next([]byte("user1a\x00user1\x00aa"))
		if err != nil {
			t.Error("Unexpected error:", err)
		}
	})
}
