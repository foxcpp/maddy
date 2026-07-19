/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

package smtpconn

import (
	"errors"
	"net"
	"testing"

	"github.com/foxcpp/maddy/framework/exterrors"
)

func dnsOpError() error {
	return &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Addr: &net.TCPAddr{
			IP:   net.ParseIP("192.0.2.1"),
			Port: 25,
		},
		Err: &net.DNSError{
			Err:        "no such host",
			Name:       "smtp.example.invalid",
			IsNotFound: true,
		},
	}
}

func TestWrapClientErrDNSErrorDefaultClassification(t *testing.T) {
	c := New()

	err := c.wrapClientErr(dnsOpError(), "smtp.example.invalid")

	var smtpErr *exterrors.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected *exterrors.SMTPError, got %T: %v", err, err)
	}

	if smtpErr.Code != 550 {
		t.Fatalf("unexpected SMTP code: got %d, want 550", smtpErr.Code)
	}

	if smtpErr.EnhancedCode != (exterrors.EnhancedCode{5, 4, 4}) {
		t.Fatalf("unexpected enhanced SMTP code: got %v, want 5.4.4", smtpErr.EnhancedCode)
	}

	if smtpErr.Temporary() {
		t.Fatal("default DNS error classification should be permanent")
	}
}

func TestWrapClientErrDNSErrorTemporaryClassification(t *testing.T) {
	c := New()
	c.DNSErrorsTemporary = true

	err := c.wrapClientErr(dnsOpError(), "smtp.example.invalid")

	var smtpErr *exterrors.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("expected *exterrors.SMTPError, got %T: %v", err, err)
	}

	if smtpErr.Code != 451 {
		t.Fatalf("unexpected SMTP code: got %d, want 451", smtpErr.Code)
	}

	if smtpErr.EnhancedCode != (exterrors.EnhancedCode{4, 4, 4}) {
		t.Fatalf("unexpected enhanced SMTP code: got %v, want 4.4.4", smtpErr.EnhancedCode)
	}

	if !smtpErr.Temporary() {
		t.Fatal("DNS error should be temporary when DNSErrorsTemporary is enabled")
	}
}
