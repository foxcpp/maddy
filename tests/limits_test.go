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

func TestConcurrencyLimit(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no
			limits {
				all concurrency 1
			}

			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	c1 := t.Conn("smtp")
	defer c1.Close()
	c1.SMTPNegotation("localhost", nil, nil)
	c1.Writeln("MAIL FROM:<testing@maddy.test")
	c1.ExpectPattern("250 *")
	// Down on semaphore.

	c2 := t.Conn("smtp")
	defer c2.Close()
	c2.SMTPNegotation("localhost", nil, nil)
	c1.Writeln("MAIL FROM:<testing@maddy.test")
	// Temporary error due to lock timeout.
	c1.ExpectPattern("451 *")
}

func TestPerIPConcurrency(tt *testing.T) {
	tt.Parallel()
	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject no
			limits {
				ip concurrency 1
			}

			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	c1 := t.Conn("smtp")
	defer c1.Close()
	c1.SMTPNegotation("localhost", nil, nil)
	c1.Writeln("MAIL FROM:<testing@maddy.test")
	c1.ExpectPattern("250 *")
	// Down on semaphore.

	c3 := t.Conn4("127.0.0.2", "smtp")
	defer c3.Close()
	c3.SMTPNegotation("localhost", nil, nil)
	c3.Writeln("MAIL FROM:<testing@maddy.test")
	c3.ExpectPattern("250 *")
	// Down on semaphore (different IP).

	c2 := t.Conn("smtp")
	defer c2.Close()
	c2.SMTPNegotation("localhost", nil, nil)
	c1.Writeln("MAIL FROM:<testing@maddy.test")
	// Temporary error due to lock timeout.
	c1.ExpectPattern("451 *")
}
