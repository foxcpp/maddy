//+build integration

package tests_test

import (
	"sync"
	"testing"
	"time"

	"github.com/foxcpp/maddy/tests"
)

func floodSmtp(c *tests.Conn, commands []string, expectedPatterns []string, iterations int) {
	for i := 0; i < iterations; i++ {
		for i, cmd := range commands {
			c.Writeln(cmd)
			if expectedPatterns[i] != "" {
				c.ExpectPattern(expectedPatterns[i])
			}
		}
	}
}

func TestSMTPFlood_FullMsg_NoLimits_1Conn(tt *testing.T) {
	tt.Parallel()

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

	c := t.Conn("smtp")
	defer c.Close()
	c.SMTPNegotation("helo.maddy.test", nil, nil)
	floodSmtp(&c, []string{
		"MAIL FROM:<from@maddy.test",
		"RCPT TO:<to@maddy.test>",
		"DATA",
		"From: <from@maddy.test>",
		"",
		"Heya!",
		".",
	}, []string{
		"250 *",
		"250 *",
		"354 *",
		"",
		"",
		"",
		"250 *",
	}, 100)
}

func TestSMTPFlood_FullMsg_NoLimits_10Conns(tt *testing.T) {
	tt.Parallel()

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

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@maddy.test",
				"RCPT TO:<to@maddy.test>",
				"DATA",
				"From: <from@maddy.test>",
				"",
				"Heya!",
				".",
			}, []string{
				"250 *",
				"250 *",
				"354 *",
				"",
				"",
				"",
				"250 *",
			}, 100)
			t.Log("Done")
		}()
	}

	wg.Wait()
}

func TestSMTPFlood_EnvelopeAbort_NoLimits_10Conns(tt *testing.T) {
	tt.Parallel()

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

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@maddy.test",
				"RCPT TO:<to@maddy.test>",
				"RSET",
			}, []string{
				"250 *",
				"250 *",
				"250 *",
			}, 100)
			t.Log("Done")
		}()
	}

	wg.Wait()
}

func TestSMTPFlood_EnvelopeAbort_Ratelimited(tt *testing.T) {
	tt.Parallel()

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			limits {
				all rate 10 1s
			}

			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conns := 5
	msgsPerConn := 10
	expectedRate := 10
	slip := 10

	start := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@maddy.test",
				"RCPT TO:<to@maddy.test>",
				"RSET",
			}, []string{
				"250 *",
				"250 *",
				"250 *",
			}, msgsPerConn)
			t.Log("Done")
		}()
	}

	wg.Wait()
	end := time.Now()

	t.Log("Sent", conns*msgsPerConn, "messages using", conns, "connections")
	t.Log("Took", end.Sub(start))

	effectiveRate := float64(conns*msgsPerConn) / end.Sub(start).Seconds()
	if effectiveRate > float64(expectedRate+slip) {
		t.Fatal("Effective rate is significantly bigger than limit:", effectiveRate)
	}
	t.Log("Effective rate:", effectiveRate)
}

func TestSMTPFlood_FullMsg_Ratelimited_PerSource(tt *testing.T) {
	tt.Parallel()

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject false

			limits {
				source rate 10 1s
			}

			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conns := 5
	msgsPerConn := 10
	expectedRate := 10
	slip := 10

	start := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@1.maddy.test",
				"RCPT TO:<to@maddy.test>",
				"DATA",
				"From: <from@1.maddy.test>",
				"",
				"Heya!",
				".",
			}, []string{
				"250 *",
				"250 *",
				"354 *",
				"",
				"",
				"",
				"250 *",
			}, msgsPerConn)
			t.Log("Done")
		}()
	}
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn("smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@2.maddy.test",
				"RCPT TO:<to@maddy.test>",
				"DATA",
				"From: <from@1.maddy.test>",
				"",
				"Heya!",
				".",
			}, []string{
				"250 *",
				"250 *",
				"354 *",
				"",
				"",
				"",
				"250 *",
			}, msgsPerConn)
			t.Log("Done")
		}()
	}

	wg.Wait()
	end := time.Now()

	t.Log("Sent", conns*msgsPerConn, "messages using", conns, "connections")
	t.Log("Took", end.Sub(start))

	effectiveRate := float64(conns*msgsPerConn*2) / end.Sub(start).Seconds()
	// Expect the rate twice since we send from two sources.
	if effectiveRate > float64(expectedRate*2+slip) {
		t.Fatal("Effective rate is significantly bigger than limit:", effectiveRate)
	}
	t.Log("Effective rate:", effectiveRate)
}

func TestSMTPFlood_EnvelopeAbort_Ratelimited_PerIP(tt *testing.T) {
	tt.Parallel()

	t := tests.NewT(tt)
	t.DNS(nil)
	t.Port("smtp")
	t.Config(`
		smtp tcp://127.0.0.1:{env:TEST_PORT_smtp} {
			hostname mx.maddy.test
			tls off

			defer_sender_reject false

			limits {
				ip rate 10 1s
			}

			deliver_to dummy
		}`)
	t.Run(1)
	defer t.Close()

	conns := 2
	msgsPerConn := 50
	expectedRate := 10
	slip := 10

	start := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn4("127.0.0.1", "smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@maddy.test",
				"RCPT TO:<to@maddy.test>",
				"RSET",
			}, []string{
				"250 *",
				"250 *",
				"250 *",
			}, msgsPerConn)
			t.Log("Done")
		}()
	}
	for i := 0; i < conns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := t.Conn4("127.0.0.2", "smtp")
			defer c.Close()
			c.SMTPNegotation("helo.maddy.test", nil, nil)
			floodSmtp(&c, []string{
				"MAIL FROM:<from@maddy.test",
				"RCPT TO:<to@maddy.test>",
				"RSET",
			}, []string{
				"250 *",
				"250 *",
				"250 *",
			}, msgsPerConn)
			t.Log("Done")
		}()
	}

	wg.Wait()
	end := time.Now()

	t.Log("Sent", 2*conns*msgsPerConn, "messages using", conns*2, "connections")
	t.Log("Took", end.Sub(start))

	effectiveRate := float64(conns*msgsPerConn*2) / end.Sub(start).Seconds()
	// Expect the rate twice since we send from two sources.
	if effectiveRate > float64(expectedRate*2+slip) {
		t.Fatal("Effective rate is significantly bigger than limit:", effectiveRate)
	}
	t.Log("Expected rate:", expectedRate*2)
	t.Log("Effective rate:", effectiveRate)
}
