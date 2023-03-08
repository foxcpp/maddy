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

package address

import (
	"testing"
)

func TestSplit(t *testing.T) {
	test := func(addr, mbox, domain string, fail bool) {
		t.Helper()

		actualMbox, actualDomain, err := Split(addr)
		if err != nil && !fail {
			t.Errorf("%s: unexpected error: %v", addr, err)
			return
		}
		if err == nil && fail {
			t.Errorf("%s: expected error, got %s, %s", addr, actualMbox, actualDomain)
			return
		}

		if actualMbox != mbox {
			t.Errorf("%s: wrong local part, want %s, got %s", addr, mbox, actualMbox)
		}
		if actualDomain != domain {
			t.Errorf("%s: wrong domain part, want %s, got %s", addr, domain, actualDomain)
		}
	}

	test("simple@example.org", "simple", "example.org", false)
	test("simple@[1.2.3.4]", "simple", "[1.2.3.4]", false)
	test("simple@[IPv6:beef::1]", "simple", "[IPv6:beef::1]", false)
	test("@example.org", "", "", true)
	test("@", "", "", true)
	test("no-domain@", "", "", true)
	test("@no-local-part", "", "", true)

	// Not a valid address, but a special value for SMTP
	// should be handled separately where necessary.
	test("", "", "", true)

	// A special SMTP value too, but permitted now.
	test("postmaster", "postmaster", "", false)
}

func TestUnquoteMbox(t *testing.T) {
	test := func(inputMbox, expectedMbox string, fail bool) {
		t.Helper()

		actualMbox, err := UnquoteMbox(inputMbox)
		if err != nil && !fail {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if err == nil && fail {
			t.Errorf("expected error, got %s", actualMbox)
			return
		}

		if actualMbox != expectedMbox {
			t.Errorf("wrong local part, want %s, got %s", actualMbox, actualMbox)
		}
	}

	test(`no\@no`, "", true)
	test("no@no", "", true)
	test(`no\"no`, "", true)
	test(`"no\"no"`, `no"no`, false)
	test(`"no@no"`, `no@no`, false)
	test(`"no no"`, `no no`, false)
	test(`"no\\no"`, `no\no`, false)
	test(`"no"no`, "", true)
	test(`postmaster`, "postmaster", false)
	test(`foo`, "foo", false)
}

func TestQuoteMbox(t *testing.T) {
	test := func(inputMbox, expectedMbox string) {
		t.Helper()

		actualMbox := QuoteMbox(inputMbox)
		if actualMbox != expectedMbox {
			t.Errorf("wrong local part, want %s, got %s", actualMbox, actualMbox)
		}
	}

	test(`no"no`, `"no\"no"`)
	test(`no@no`, `"no@no"`)
	test(`no no`, `"no no"`)
	test(`no\no`, `"no\\no"`)
	test("postmaster", `postmaster`)
	test("foo", `foo`)
}
