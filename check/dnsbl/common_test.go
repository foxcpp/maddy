package dnsbl

import (
	"net"
	"testing"
)

// TODO: Tests for checkIP and checkDomain once we have proper DNS mocking.

func TestQueryString(t *testing.T) {
	test := func(ip, queryStr string) {
		t.Helper()

		parsed := net.ParseIP(ip)
		if parsed == nil {
			panic("Malformed IP in test")
		}

		actual := queryString(parsed)
		if actual != queryStr {
			t.Errorf("want queryString(%s) to be %s, got %s", ip, queryStr, actual)
		}
	}

	test("2001:db8:1:2:3:4:567:89ab", "b.a.9.8.7.6.5.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.8.b.d.0.1.0.0.2")
	test("2001::1:2:3:4:567:89ab", "b.a.9.8.7.6.5.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.0.0.0.0.1.0.0.2")
	test("192.0.2.99", "99.2.0.192")
}
