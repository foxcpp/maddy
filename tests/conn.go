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

package tests

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
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
func (c *Conn) Write(s string) {
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
		c.fatal("Unexpected I/O error: %v", err)
	}
}

func (c *Conn) Writeln(s string) {
	c.T.Helper()

	c.Write(s + "\r\n")
}

func (c *Conn) Readln() (string, error) {
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

func (c *Conn) Expect(line string) {
	c.T.Helper()

	actual, err := c.Readln()
	if err != nil {
		c.T.Fatal("Unexpected I/O error:", err)
	}

	if line != actual {
		c.T.Fatalf("Response line not matching the expected one, want %q", line)
	}
}

// ExpectPattern reads a line from the connection socket and checks whether is
// matches the supplied shell pattern (as defined by path.Match). The original
// line is returned.
func (c *Conn) ExpectPattern(pat string) string {
	c.T.Helper()

	line, err := c.Readln()
	if err != nil {
		c.T.Fatal("Unexpected I/O error:", err)
	}

	match, err := path.Match(pat, line)
	if err != nil {
		c.T.Fatal("Malformed pattern:", err)
	}
	if !match {
		c.T.Fatalf("Response line not matching the expected pattern, want %q", pat)
	}

	return line
}

func (c *Conn) fatal(f string, args ...interface{}) {
	c.T.Helper()
	c.log('-', f, args...)
	c.T.FailNow()
}

func (c *Conn) log(direction rune, f string, args ...interface{}) {
	c.T.Helper()

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

func (c *Conn) SMTPPlainAuth(username, password string, expectOk bool) {
	c.T.Helper()

	resp := append([]byte{0x00}, username...)
	resp = append(resp, 0x00)
	resp = append(resp, password...)
	c.Writeln("AUTH PLAIN " + base64.StdEncoding.EncodeToString(resp))
	if expectOk {
		c.ExpectPattern("235 *")
	} else {
		c.ExpectPattern("*")
	}
}

func (c *Conn) SMTPNegotation(ourName string, requireExts, blacklistExts []string) {
	c.T.Helper()

	needCapsMap := make(map[string]bool)
	blacklistCapsMap := make(map[string]bool)
	for _, ext := range requireExts {
		needCapsMap[ext] = false
	}
	for _, ext := range blacklistExts {
		blacklistCapsMap[ext] = false
	}

	c.Writeln("EHLO " + ourName)

	// Consume the first line from socket, it is either initial greeting (sent
	// before we sent EHLO) or the EHLO reply in case of re-negotiation after
	// STARTTLS.
	l, err := c.Readln()
	if err != nil {
		c.T.Fatal("I/O error during SMTP negotiation:", err)
	}
	if strings.HasPrefix(l, "220") {
		// That was initial greeting, consume one more line.
		c.ExpectPattern("250-*")
	}

	var caps []string
capsloop:
	for {
		line, err := c.Readln()
		if err != nil {
			c.T.Fatal("I/O error during SMTP negotiation:", err)
		}

		switch {
		case strings.HasPrefix(line, "250-"):
			caps = append(caps, strings.TrimPrefix(line, "250-"))
		case strings.HasPrefix(line, "250 "):
			caps = append(caps, strings.TrimPrefix(line, "250 "))
			break capsloop
		default:
			c.T.Fatal("Unexpected reply during SMTP negotiation:", line)
		}
	}

	for _, ext := range caps {
		needCapsMap[ext] = true
		if _, ok := blacklistCapsMap[ext]; ok {
			blacklistCapsMap[ext] = true
		}
	}
	for ext, status := range needCapsMap {
		if !status {
			c.T.Fatalf("Capability %v is missing but required", ext)
		}
	}
	for ext, status := range blacklistCapsMap {
		if status {
			c.T.Fatalf("Capability %v is present but not allowed", ext)
		}
	}
}

func (c *Conn) Close() error {
	return c.Conn.Close()
}

func (c *Conn) Rebind(subtest *T) *Conn {
	cpy := *c
	cpy.T = subtest
	return &cpy
}
