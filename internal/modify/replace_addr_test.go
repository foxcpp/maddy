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
	"reflect"
	"testing"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func testReplaceAddr(t *testing.T, modName string) {
	test := func(addr string, expectedMulti []string, aliases map[string][]string) {
		t.Helper()

		mod, err := NewReplaceAddr(modName, "", nil, []string{"dummy"})
		if err != nil {
			t.Fatal(err)
		}
		m := mod.(*replaceAddr)
		if err := m.Init(config.NewMap(nil, config.Node{})); err != nil {
			t.Fatal(err)
		}
		m.table = testutils.MultiTable{M: aliases}

		var actualMulti []string
		if modName == "modify.replace_sender" {
			var actual string
			actual, err = m.RewriteSender(context.Background(), addr)
			if err != nil {
				t.Fatal(err)
			}
			actualMulti = []string{actual}
		}
		if modName == "modify.replace_rcpt" {
			actualMulti, err = m.RewriteRcpt(context.Background(), addr)
			if err != nil {
				t.Fatal(err)
			}
		}

		if !reflect.DeepEqual(actualMulti, expectedMulti) {
			t.Errorf("want %s, got %s", expectedMulti, actualMulti)
		}
	}

	test("test@example.org", []string{"test@example.org"}, nil)
	test("postmaster", []string{"postmaster"}, nil)
	test("test@example.com", []string{"test@example.org"},
		map[string][]string{"test@example.com": []string{"test@example.org"}})
	test(`"\"test @ test\""@example.com`, []string{"test@example.org"},
		map[string][]string{`"\"test @ test\""@example.com`: []string{"test@example.org"}})
	test(`test@example.com`, []string{`"\"test @ test\""@example.org`},
		map[string][]string{`test@example.com`: []string{`"\"test @ test\""@example.org`}})
	test(`"\"test @ test\""@example.com`, []string{`"\"b @ b\""@example.com`},
		map[string][]string{`"\"test @ test\""`: []string{`"\"b @ b\""`}})
	test("TeSt@eXAMple.com", []string{"test@example.org"},
		map[string][]string{"test@example.com": []string{"test@example.org"}})
	test("test@example.com", []string{"test2@example.com"},
		map[string][]string{"test": []string{"test2"}})
	test("test@example.com", []string{"test2@example.org"},
		map[string][]string{"test": []string{"test2@example.org"}})
	test("postmaster", []string{"test2@example.org"},
		map[string][]string{"postmaster": []string{"test2@example.org"}})
	test("TeSt@examPLE.com", []string{"test2@example.com"},
		map[string][]string{"test": []string{"test2"}})
	test("test@example.com", []string{"test3@example.com"},
		map[string][]string{
			"test@example.com": []string{"test3@example.com"},
			"test":             []string{"test2"},
		})
	test("rcpt@E\u0301.example.com", []string{"rcpt@foo.example.com"},
		map[string][]string{
			"rcpt@\u00E9.example.com": []string{"rcpt@foo.example.com"},
		})
	test("E\u0301@foo.example.com", []string{"rcpt@foo.example.com"},
		map[string][]string{
			"\u00E9@foo.example.com": []string{"rcpt@foo.example.com"},
		})

	if modName == "modify.replace_rcpt" {
		//multiple aliases
		test("test@example.com", []string{"test@example.org", "test@example.net"},
			map[string][]string{"test@example.com": []string{"test@example.org", "test@example.net"}})
		test("test@example.com", []string{"1@example.com", "2@example.com", "3@example.com"},
			map[string][]string{"test@example.com": []string{"1@example.com", "2@example.com", "3@example.com"}})
	}
}

func TestReplaceAddr_RewriteSender(t *testing.T) {
	testReplaceAddr(t, "modify.replace_sender")
}

func TestReplaceAddr_RewriteRcpt(t *testing.T) {
	testReplaceAddr(t, "modify.replace_rcpt")
}
