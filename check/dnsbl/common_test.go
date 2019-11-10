package dnsbl

import (
	"net"
	"reflect"
	"testing"

	"github.com/foxcpp/go-mockdns"
)

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

func TestCheckDomain(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, cfg List, domain string, expectedErr error) {
		t.Helper()
		resolver := mockdns.Resolver{Zones: zones}
		err := checkDomain(&resolver, cfg, domain)
		if !reflect.DeepEqual(err, expectedErr) {
			t.Errorf("expected err to be '%#v', got '%#v'", expectedErr, err)
		}
	}

	test(nil, List{Zone: "example.org"}, "example.com", nil)
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			Err: &net.DNSError{
				Err:         "i/o timeout",
				IsTimeout:   true,
				IsTemporary: true,
			},
		},
	}, List{Zone: "example.org"}, "example.com", &net.DNSError{
		Err:         "i/o timeout",
		IsTimeout:   true,
		IsTemporary: true,
	})
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org"}, "example.com", ListedErr{
		Identity: "example.com",
		List:     "example.org",
		Reason:   "127.0.0.1",
	})
	test(map[string]mockdns.Zone{
		"example.org.example.com.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org"}, "example.com", nil)
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A:   []string{"127.0.0.1"},
			TXT: []string{"Reason"},
		},
	}, List{Zone: "example.org"}, "example.com", ListedErr{
		Identity: "example.com",
		List:     "example.org",
		Reason:   "Reason",
	})
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A:   []string{"127.0.0.1"},
			TXT: []string{"Reason 1", "Reason 2"},
		},
	}, List{Zone: "example.org"}, "example.com", ListedErr{
		Identity: "example.com",
		List:     "example.org",
		Reason:   "Reason 1; Reason 2",
	})
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A: []string{"127.0.0.1", "127.0.0.2"},
		},
	}, List{Zone: "example.org"}, "example.com", ListedErr{
		Identity: "example.com",
		List:     "example.org",
		Reason:   "127.0.0.1; 127.0.0.2",
	})
}

func TestCheckIP(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, cfg List, ip net.IP, expectedErr error) {
		t.Helper()
		resolver := mockdns.Resolver{Zones: zones}
		err := checkIP(&resolver, cfg, ip)
		if !reflect.DeepEqual(err, expectedErr) {
			t.Errorf("expected err to be '%#v', got '%#v'", expectedErr, err)
		}
	}

	test(nil, List{Zone: "example.org"}, net.IPv4(1, 2, 3, 4), nil)
	test(nil, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), nil)
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Reason:   "127.0.0.1",
	})
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org"}, net.IPv4(1, 2, 3, 4), nil)
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			Err: &net.DNSError{
				Err:         "i/o timeout",
				IsTimeout:   true,
				IsTemporary: true,
			},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), &net.DNSError{
		Err:         "i/o timeout",
		IsTimeout:   true,
		IsTemporary: true,
	})

	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A:   []string{"127.0.0.1"},
			TXT: []string{"Reason"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Reason:   "Reason",
	})
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1", "127.0.0.2"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Reason:   "127.0.0.1; 127.0.0.2",
	})
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A:   []string{"127.0.0.1", "127.0.0.2"},
			TXT: []string{"Reason", "Reason 2"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Reason:   "Reason; Reason 2",
	})
	test(map[string]mockdns.Zone{
		"b.a.9.8.7.6.5.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.8.b.d.0.1.0.0.2.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.ParseIP("2001:db8:1:2:3:4:567:89ab"), nil)
	test(map[string]mockdns.Zone{
		"b.a.9.8.7.6.5.0.4.0.0.0.3.0.0.0.2.0.0.0.1.0.0.0.8.b.d.0.1.0.0.2.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", ClientIPv6: true}, net.ParseIP("2001:db8:1:2:3:4:567:89ab"), ListedErr{
		Identity: "2001:db8:1:2:3:4:567:89ab",
		List:     "example.org",
		Reason:   "127.0.0.1",
	})
}
