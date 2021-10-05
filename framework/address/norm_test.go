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

func addrFuncTest(t *testing.T, f func(string) (string, error)) func(in, wantOut string, fail bool) {
	return func(in, wantOut string, fail bool) {
		t.Helper()

		out, err := f(in)
		if err != nil {
			if !fail {
				t.Errorf("Expected failure, got none")
			}
		}
		if out != wantOut {
			t.Errorf("Wrong result: want '%s', got '%s'", wantOut, out)
		}
	}
}

func TestForLookup(t *testing.T) {
	test := addrFuncTest(t, ForLookup)
	test("test@example.org", "test@example.org", false)
	test("E\u0301@example.org", "\u00E9@example.org", false)
	test("test@EXAMPLE.org", "test@example.org", false)
	test("test@xn--e1aybc.example.org", "test@тест.example.org", false)
	test("TEST@xn--99999999999.example.org", "test@xn--99999999999.example.org", true)
	test("tESt@", "test@", true)
	test("postmaster", "postmaster", false)
}

func TestCleanDomain(t *testing.T) {
	test := addrFuncTest(t, CleanDomain)
	test("test@example.org", "test@example.org", false)
	test("whateveR@example.org", "whateveR@example.org", false)
	test("E\u0301@example.org", "E\u0301@example.org", false)
	test("test@EXAMPLE.org", "test@example.org", false)
	test("test@xn--e1aybc.example.org", "test@тест.example.org", false)
	test("TEST@xn--99999999999.example.org", "TEST@xn--99999999999.example.org", true)
	test("tESt@", "tESt@", true)
	test("postmaster", "postmaster", false)
}

func TestEqual(t *testing.T) {
	test := func(in1, in2 string, wantEq bool) {
		eq := Equal(in1, in2)
		if eq != wantEq {
			t.Errorf("Want Equal(%s, %s) == %v, got %v", in1, in2, wantEq, eq)
		}
	}

	test("test@example.org", "test@example.org", true)
	test("test2@example.org", "test@example.org", false)
	test("TEST2@example.org", "TesT2@example.org", true)
	test("E\u0301@example.org", "\u00E9@example.org", true)
	test("test@тест.example.org", "test@xn--e1aybc.example.org", true)
	test("test@xn--999999999999999.example.org", "test@xn--999999999999999.example.org", true)
	test("test@xn--999999999999.example.org", "test@xn--999999999999999.example.org", false)
}

func TestIsASCII(t *testing.T) {
	if !IsASCII("hello") {
		t.Errorf("'hello' is ASCII")
	}
	if IsASCII("тест") {
		t.Errorf("'тест' is non-ASCII")
	}
}
