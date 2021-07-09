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

package modify

import (
	"context"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func testReplaceAddr(t *testing.T, modName string, rewriter func(*replaceAddr, context.Context, string) (string, error)) {
	test := func(addr, expected string, aliases map[string]string) {
		t.Helper()

		mod, err := NewReplaceAddr(modName, "", nil, []string{"dummy"})
		if err != nil {
			t.Fatal(err)
		}
		m := mod.(*replaceAddr)
		if err := m.Init(config.NewMap(nil, config.Node{})); err != nil {
			t.Fatal(err)
		}
		m.table = testutils.Table{M: aliases}

		var actual string
		if modName == "modify.replace_sender" {
			actual, err = m.RewriteSender(context.Background(), addr)
			if err != nil {
				t.Fatal(err)
			}
		}
		if modName == "modify.replace_rcpt" {
			actual, err = m.RewriteRcpt(context.Background(), addr)
			if err != nil {
				t.Fatal(err)
			}
		}

		if actual != expected {
			t.Errorf("want %s, got %s", expected, actual)
		}
	}

	test("test@example.org", "test@example.org", nil)
	test("postmaster", "postmaster", nil)
	test("test@example.com", "test@example.org",
		map[string]string{"test@example.com": "test@example.org"})
	test(`"\"test @ test\""@example.com`, "test@example.org",
		map[string]string{`"\"test @ test\""@example.com`: "test@example.org"})
	test(`test@example.com`, `"\"test @ test\""@example.org`,
		map[string]string{`test@example.com`: `"\"test @ test\""@example.org`})
	test(`"\"test @ test\""@example.com`, `"\"b @ b\""@example.com`,
		map[string]string{`"\"test @ test\""`: `"\"b @ b\""`})
	test("TeSt@eXAMple.com", "test@example.org",
		map[string]string{"test@example.com": "test@example.org"})
	test("test@example.com", "test2@example.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test2@example.org",
		map[string]string{"test": "test2@example.org"})
	test("postmaster", "test2@example.org",
		map[string]string{"postmaster": "test2@example.org"})
	test("TeSt@examPLE.com", "test2@example.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test3@example.com",
		map[string]string{
			"test@example.com": "test3@example.com",
			"test":             "test2",
		})
	test("rcpt@E\u0301.example.com", "rcpt@foo.example.com",
		map[string]string{
			"rcpt@\u00E9.example.com": "rcpt@foo.example.com",
		})
	test("E\u0301@foo.example.com", "rcpt@foo.example.com",
		map[string]string{
			"\u00E9@foo.example.com": "rcpt@foo.example.com",
		})
}

func TestReplaceAddr_RewriteSender(t *testing.T) {
	testReplaceAddr(t, "modify.replace_sender", (*replaceAddr).RewriteSender)
}

func TestReplaceAddr_RewriteRcpt(t *testing.T) {
	testReplaceAddr(t, "modify.replace_rcpt", (*replaceAddr).RewriteRcpt)
}
