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
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestAuth_AuthPlain(t *testing.T) {
	addSHA256()

	mod, err := New("pass_table", "", nil, []string{"dummy"})
	if err != nil {
		t.Fatal(err)
	}
	err = mod.Init(config.NewMap(nil, config.Node{
		Children: []config.Node{},
	}))
	if err != nil {
		t.Fatal(err)
	}
	a := mod.(*Auth)
	a.table = testutils.Table{
		M: map[string]string{
			"foxcpp":       "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw=",
			"not-foxcpp":   "bcrypt:$2y$10$4tEJtJ6dApmhETg8tJ4WHOeMtmYXQwmHDKIyfg09Bw1F/smhLjlaa",
			"not-foxcpp-2": "argon2:1:8:1:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q==",
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
}
