package dnsbl

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestCheckList(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, cfg List, ip net.IP, ehlo, mailFrom string, expectedErr error) {
		mod := &DNSBL{
			resolver: &mockdns.Resolver{Zones: zones},
			log:      testutils.Logger(t, "dnsbl"),
		}
		err := mod.checkList(context.Background(), cfg, ip, ehlo, mailFrom)
		if !reflect.DeepEqual(err, expectedErr) {
			t.Errorf("expected err to be '%#v', got '%#v'", expectedErr, err)
		}
	}

	test(nil, List{Zone: "example.org"}, net.IPv4(1, 2, 3, 4),
		"example.com", "foo@example.com", nil)
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", ClientIPv4: true}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", ListedErr{
			Identity: "1.2.3.4",
			List:     "example.org",
			Reason:   "127.0.0.1",
		},
	)
	test(map[string]mockdns.Zone{
		"mx.example.com.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org"}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", nil,
	)
	test(map[string]mockdns.Zone{
		"mx.example.com.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", EHLO: true}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", ListedErr{
			Identity: "mx.example.com",
			List:     "example.org",
			Reason:   "127.0.0.1",
		},
	)
	test(map[string]mockdns.Zone{
		"[1.2.3.4].example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", EHLO: true}, net.IPv4(1, 2, 3, 4),
		"[1.2.3.4]", "foo@example.com", nil,
	)
	test(map[string]mockdns.Zone{
		"[IPv6:beef::1].example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", EHLO: true}, net.IPv4(1, 2, 3, 4),
		"[IPv6:beef::1]", "foo@example.com", nil,
	)
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org"}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", nil,
	)
	test(map[string]mockdns.Zone{
		"postmaster.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", MAILFROM: true}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "postmaster", nil,
	)
	test(map[string]mockdns.Zone{
		".example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", MAILFROM: true}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "", nil,
	)
	test(map[string]mockdns.Zone{
		"example.com.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, List{Zone: "example.org", MAILFROM: true}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", ListedErr{
			Identity: "example.com",
			List:     "example.org",
			Reason:   "127.0.0.1",
		},
	)
}

func TestCheckLists(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, bls []List, ip net.IP, ehlo, mailFrom string, reject, quarantine bool) {
		mod := &DNSBL{
			bls:             bls,
			resolver:        &mockdns.Resolver{Zones: zones},
			log:             testutils.Logger(t, "dnsbl"),
			quarantineThres: 1,
			rejectThres:     2,
		}
		result := mod.checkLists(context.Background(), ip, ehlo, mailFrom)

		if result.Reject && !reject {
			t.Errorf("Expected message to not be rejected")
		}
		if !result.Reject && reject {
			t.Errorf("Expected message to be rejected")
		}
		if result.Quarantine && !quarantine {
			t.Errorf("Expected message to not be quarantined")
		}
		if !result.Quarantine && quarantine {
			t.Errorf("Expected message to be quarantined")
		}
	}

	// Score 2 >= 2, reject
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, []List{
		{
			Zone:       "example.org",
			ClientIPv4: true,
			ScoreAdj:   2,
		},
	}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", true, false,
	)

	// Score 1 >= 1, quarantine
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, []List{
		{
			Zone:       "example.org",
			ClientIPv4: true,
			ScoreAdj:   1,
		},
	}, net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com", false, true,
	)

	// Score 0, no action
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
		"4.3.2.1.example.net.": {
			A: []string{"127.0.0.1"},
		},
	},
		[]List{
			{Zone: "example.org", ClientIPv4: true, ScoreAdj: 1},
			{Zone: "example.net", ClientIPv4: true, ScoreAdj: -1},
		},
		net.IPv4(1, 2, 3, 4),
		"mx.example.com", "foo@example.com",
		false, false,
	)

	// DNS error, hard-fail (reject)
	test(map[string]mockdns.Zone{
		"4.3.2.2.example.org.": {
			Err: &net.DNSError{
				Err:         "i/o timeout",
				IsTimeout:   true,
				IsTemporary: true,
			},
		},
	},
		[]List{
			{Zone: "example.org", ClientIPv4: true, ScoreAdj: 1},
			{Zone: "example.net", ClientIPv4: true, ScoreAdj: 2},
		},
		net.IPv4(2, 2, 3, 4),
		"mx.example.com", "foo@example.com",
		true, false,
	)
}
