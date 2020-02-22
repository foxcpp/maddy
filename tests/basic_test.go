//+build integration

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
	conn.ExpectPattern("250-SMTPUTF8")
	conn.ExpectPattern("250 SIZE *")
	conn.Writeln("QUIT")
	conn.ExpectPattern("221 *")
}
