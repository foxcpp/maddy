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

package plain_separate

import (
	"context"
	"errors"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/framework/module"
)

type mockAuth struct {
	db map[string]bool
}

func (mockAuth) SASLMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

func (m mockAuth) AuthPlain(username, _ string) error {
	ok := m.db[username]
	if !ok {
		return errors.New("invalid creds")
	}
	return nil
}

type mockTable struct {
	db map[string]string
}

func (m mockTable) Lookup(_ context.Context, a string) (string, bool, error) {
	b, ok := m.db[a]
	return b, ok, nil
}

func TestPlainSplit_NoUser(t *testing.T) {
	a := Auth{
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_NoUser_MultiPass(t *testing.T) {
	a := Auth{
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_UserPass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"user1": "",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_MultiUser_Pass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"userWH": "",
				},
			},
			mockTable{
				db: map[string]string{
					"user1": "",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}
