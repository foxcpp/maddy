//go:build integration
// +build integration

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

package tests_test

import (
	"testing"

	"github.com/foxcpp/maddy/tests"
)

func TestBasic(tt *testing.T) {
	tt.Parallel()

	// This test is mostly intended to test whether the integration testing
	// library is working as expected.

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conn := t.Conn("smtp")
	defer conn.Close()
	conn.ExpectPattern("220 mx.maddy.test *")
	conn.Writeln("EHLO localhost")
	conn.ExpectPattern("250-*")
	conn.ExpectPattern("250-PIPELINING")
	conn.ExpectPattern("250-8BITMIME")
	conn.ExpectPattern("250-ENHANCEDSTATUSCODES")
	conn.ExpectPattern("250-CHUNKING")
	conn.ExpectPattern("250-SMTPUTF8")
	conn.ExpectPattern("250 SIZE *")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}
