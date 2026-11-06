//go:build integration

package tests_test

import (
	"fmt"
	"testing"

	"github.com/foxcpp/maddy/tests"
)

func TestSourceOnlyLimitDoesNotPanicWhenBucketSetIsFull(tt *testing.T) {
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
				source concurrency 1
			}

			deliver_to dummy
		}
	`)
	t.Run(1)
	defer t.Close()

	c := t.Conn("smtp")
	defer c.MustClose()
	c.SMTPNegotation("localhost", nil, nil)

	for i := 0; i <= 20010; i++ {
		c.Writeln(fmt.Sprintf("MAIL FROM:<sender@source-%d.test>", i))
		c.ExpectPattern("250 *")
		c.Writeln("RSET")
		c.ExpectPattern("250 *")
	}

	// The next distinct source exhausts the source bucket set. It must be
	// rejected normally rather than panicking while cleaning up the absent IP
	// limiter.
	c.Writeln("MAIL FROM:<sender@source-overflow.test>")
	c.ExpectPattern("554 *")

	// A process that survived the rejected message must still accept new SMTP
	// connections.
	c2 := t.Conn("smtp")
	defer c2.MustClose()
	c2.SMTPNegotation("localhost", nil, nil)
}
