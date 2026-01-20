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

package dnsbl

import (
	"context"
	"errors"
	"net"
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
		if !errors.Is(err, expectedErr) {
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

func TestCheckIPWithResponseRules(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, cfg List, ip net.IP, expectedErr error) {
		t.Helper()
		resolver := mockdns.Resolver{Zones: zones}
		err := checkIP(context.Background(), &resolver, cfg, ip)
		if expectedErr == nil {
			if err != nil {
				t.Errorf("expected no error, got '%#v'", err)
			}
		} else {
			if err == nil {
				t.Errorf("expected err to be '%#v', got nil", expectedErr)
			} else {
				expectedLE, okExpected := expectedErr.(ListedErr)
				actualLE, okActual := err.(ListedErr)
				if !okExpected || !okActual {
					t.Errorf("expected err to be '%#v', got '%#v'", expectedErr, err)
				} else {
					if expectedLE.Identity != actualLE.Identity ||
						expectedLE.List != actualLE.List ||
						expectedLE.Score != actualLE.Score ||
						expectedLE.Message != actualLE.Message {
						t.Errorf("expected err to be '%#v', got '%#v'", expectedErr, err)
					}
				}
			}
		}
	}

	// Test single response code with score and message
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.2"},
		},
	}, List{
		Zone:       "example.org",
		ClientIPv4: true,
		ResponseRules: []ResponseRule{
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   10,
				Message: "Listed in SBL",
			},
		},
	}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Score:    10,
		Message:  "Listed in SBL",
	})

	// Test multiple response codes with different scores - scores should sum
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.2", "127.0.0.11"},
		},
	}, List{
		Zone:       "example.org",
		ClientIPv4: true,
		ResponseRules: []ResponseRule{
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
					{IP: net.IPv4(127, 0, 0, 3), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   10,
				Message: "Listed in SBL",
			},
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 10), Mask: net.IPv4Mask(255, 255, 255, 255)},
					{IP: net.IPv4(127, 0, 0, 11), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   5,
				Message: "Listed in PBL",
			},
		},
	}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Score:    15, // 10 + 5
		Message:  "Listed in SBL",
	})

	// Test response code that doesn't match any rule - should return nil
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.99"},
		},
	}, List{
		Zone:       "example.org",
		ClientIPv4: true,
		ResponseRules: []ResponseRule{
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   10,
				Message: "Listed in SBL",
			},
		},
	}, net.IPv4(1, 2, 3, 4), nil)

	// Test low severity only - should get score 5
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.10"},
		},
	}, List{
		Zone:       "example.org",
		ClientIPv4: true,
		ResponseRules: []ResponseRule{
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   10,
				Message: "Listed in SBL",
			},
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 10), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   5,
				Message: "Listed in PBL",
			},
		},
	}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Score:    5,
		Message:  "Listed in PBL",
	})

	// Test high severity - should get score 10
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.2"},
		},
	}, List{
		Zone:       "example.org",
		ClientIPv4: true,
		ResponseRules: []ResponseRule{
			{
				Networks: []net.IPNet{
					{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
				},
				Score:   10,
				Message: "Listed in SBL",
			},
		},
	}, net.IPv4(1, 2, 3, 4), ListedErr{
		Identity: "1.2.3.4",
		List:     "example.org",
		Score:    10,
		Message:  "Listed in SBL",
	})
}

func TestCheckListsWithResponseRules(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, bls []List, ip net.IP, ehlo, mailFrom string, reject, quarantine bool) {
		mod := &DNSBL{
			bls:             bls,
			resolver:        &mockdns.Resolver{Zones: zones},
			log:             testutils.Logger(t, "dnsbl"),
			quarantineThres: 5,
			rejectThres:     10,
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

	// Test: Only low-severity code returned -> quarantine but not reject
	test(map[string]mockdns.Zone{
		"4.3.2.1.zen.example.org.": {
			A: []string{"127.0.0.11"},
		},
	}, []List{
		{
			Zone:       "zen.example.org",
			ClientIPv4: true,
			ResponseRules: []ResponseRule{
				{
					Networks: []net.IPNet{
						{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
					},
					Score:   10,
					Message: "Listed in SBL",
				},
				{
					Networks: []net.IPNet{
						{IP: net.IPv4(127, 0, 0, 10), Mask: net.IPv4Mask(255, 255, 255, 255)},
						{IP: net.IPv4(127, 0, 0, 11), Mask: net.IPv4Mask(255, 255, 255, 255)},
					},
					Score:   5,
					Message: "Listed in PBL",
				},
			},
		},
	}, net.IPv4(1, 2, 3, 4), "mx.example.com", "foo@example.com", false, true)

	// Test: High-severity code returned -> reject
	test(map[string]mockdns.Zone{
		"4.3.2.1.zen.example.org.": {
			A: []string{"127.0.0.2"},
		},
	}, []List{
		{
			Zone:       "zen.example.org",
			ClientIPv4: true,
			ResponseRules: []ResponseRule{
				{
					Networks: []net.IPNet{
						{IP: net.IPv4(127, 0, 0, 2), Mask: net.IPv4Mask(255, 255, 255, 255)},
					},
					Score:   10,
					Message: "Listed in SBL",
				},
			},
		},
	}, net.IPv4(1, 2, 3, 4), "mx.example.com", "foo@example.com", true, false)

	// Test: Legacy configuration without response blocks -> existing behavior preserved
	test(map[string]mockdns.Zone{
		"4.3.2.1.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, []List{
		{
			Zone:       "example.org",
			ClientIPv4: true,
			ScoreAdj:   10,
		},
	}, net.IPv4(1, 2, 3, 4), "mx.example.com", "foo@example.com", true, false)

	// Test: Mixed configuration (some lists with response blocks, some without) -> both work correctly
	test(map[string]mockdns.Zone{
		"4.3.2.1.zen.example.org.": {
			A: []string{"127.0.0.11"},
		},
		"4.3.2.1.legacy.example.org.": {
			A: []string{"127.0.0.1"},
		},
	}, []List{
		{
			Zone:       "zen.example.org",
			ClientIPv4: true,
			ResponseRules: []ResponseRule{
				{
					Networks: []net.IPNet{
						{IP: net.IPv4(127, 0, 0, 11), Mask: net.IPv4Mask(255, 255, 255, 255)},
					},
					Score:   5,
					Message: "Listed in PBL",
				},
			},
		},
		{
			Zone:       "legacy.example.org",
			ClientIPv4: true,
			ScoreAdj:   3,
		},
	}, net.IPv4(1, 2, 3, 4), "mx.example.com", "foo@example.com", false, true) // 5 + 3 = 8, quarantine but not reject
}
