//+build integration

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
