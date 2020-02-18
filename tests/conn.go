package tests

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"time"
)

// Conn is a helper that simplifies testing of text protocol interactions.
type Conn struct {
	T *T

	WriteTimeout time.Duration
	ReadTimeout  time.Duration

	allowIOErr bool

	Conn    net.Conn
	Scanner *bufio.Scanner
}

// AllowIOErr toggles whether I/O errors should be returned to the caller of
// Conn method or should immedately fail the test.
//
// By default (ok = false), the latter happens.
func (c *Conn) AllowIOErr(ok bool) {
	c.allowIOErr = ok
}

// Write writes the string to the connection socket.
func (c *Conn) Write(s string) error {
	c.T.Helper()

	// Make sure the test will not accidentally hang waiting for I/O forever if
	// the server breaks.
	if err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
		c.fatal("Cannot set write deadline: %v", err)
	}
	defer func() {
		if err := c.Conn.SetWriteDeadline(time.Time{}); err != nil {
			c.log('-', "Failed to reset connection deadline: %v", err)
		}
	}()

	c.log('>', "%s", s)
	if _, err := io.WriteString(c.Conn, s); err != nil {
		if c.allowIOErr {
			return err
		}
		c.fatal("Unexpected I/O error: %v", err)
	}

	return nil
}

func (c *Conn) Writeln(s string) error {
	c.T.Helper()

	return c.Write(s + "\r\n")
}

func (c *Conn) consumeLine() (string, error) {
	c.T.Helper()

	// Make sure the test will not accidentally hang waiting for I/O forever if
	// the server breaks.
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
		c.fatal("Cannot set write deadline: %v", err)
	}
	defer func() {
		if err := c.Conn.SetReadDeadline(time.Time{}); err != nil {
			c.log('-', "Failed to reset connection deadline: %v", err)
		}
	}()

	if !c.Scanner.Scan() {
		if err := c.Scanner.Err(); err != nil {
			if c.allowIOErr {
				return "", err
			}
			c.fatal("Unexpected I/O error: %v", err)
		}
		if c.allowIOErr {
			return "", io.EOF
		}
		c.fatal("Unexpected EOF")
	}

	c.log('<', "%v", c.Scanner.Text())

	return c.Scanner.Text(), nil
}

// ExpectPattern reads a line from the connection socket and checks whether is
// matches the supplied shell pattern (as defined by path.Match). The original
// line is returned.
func (c *Conn) ExpectPattern(pat string) (string, error) {
	c.T.Helper()

	line, err := c.consumeLine()
	if err != nil {
		return line, err
	}

	match, err := path.Match(pat, line)
	if err != nil {
		c.T.Fatal("Malformed pattern:", err)
	}
	if !match {
		c.T.Fatal("Response line not matching the expected pattern, want", pat)
	}

	return line, nil
}

func (c *Conn) fatal(f string, args ...interface{}) {
	c.log('-', f, args...)
	c.T.FailNow()
}

func (c *Conn) error(f string, args ...interface{}) {
	c.log('-', f, args...)
	c.T.Fail()
}

func (c *Conn) log(direction rune, f string, args ...interface{}) {
	local, remote := c.Conn.LocalAddr().(*net.TCPAddr), c.Conn.RemoteAddr().(*net.TCPAddr)
	msg := strings.Builder{}
	if local.IP.IsLoopback() {
		msg.WriteString(strconv.Itoa(local.Port))
	} else {
		msg.WriteString(local.String())
	}

	msg.WriteRune(' ')
	msg.WriteRune(direction)
	msg.WriteRune(' ')

	if remote.IP.IsLoopback() {
		textPort := c.T.portsRev[uint16(remote.Port)]
		if textPort != "" {
			msg.WriteString(textPort)
		} else {
			msg.WriteString(strconv.Itoa(remote.Port))
		}

	} else {
		msg.WriteString(local.String())
	}

	if _, ok := c.Conn.(*tls.Conn); ok {
		msg.WriteString(" [tls]")
	}
	msg.WriteString(": ")
	fmt.Fprintf(&msg, f, args...)
	c.T.Log(strings.TrimRight(msg.String(), "\r\n "))
}

func (c *Conn) TLS() {
	c.T.Helper()

	tlsC := tls.Client(c.Conn, &tls.Config{
		ServerName:         "maddy.test",
		InsecureSkipVerify: true,
	})
	if err := tlsC.Handshake(); err != nil {
		c.fatal("TLS handshake fail: %v", err)
	}

	c.Conn = tlsC
	c.Scanner = bufio.NewScanner(c.Conn)
}

func (c *Conn) Close() error {
	return c.Conn.Close()
}
