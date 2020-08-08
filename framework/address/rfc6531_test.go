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
	"strings"
	"testing"
)

func TestToASCII(t *testing.T) {
	test := addrFuncTest(t, ToASCII)
	test("test@тест.example.org", "test@xn--e1aybc.example.org", false)
	test("test@org."+strings.Repeat("x", 65535)+"\uFF00", "test@org."+strings.Repeat("x", 65535)+"\uFF00", true)
	test("тест@example.org", "тест@example.org", true)
	test("postmaster", "postmaster", false)
	test("postmaster@", "postmaster@", true)
}

func TestToUnicode(t *testing.T) {
	test := addrFuncTest(t, ToUnicode)
	test("test@xn--e1aybc.example.org", "test@тест.example.org", false)
	test("test@xn--9999999999999999999a.org", "test@xn--9999999999999999999a.org", true)
	test("postmaster", "postmaster", false)
	test("postmaster@", "postmaster@", true)
}
