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

package dns

import (
	"net"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/future"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/check"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestRequireMatchingRDNS(t *testing.T) {
	test := func(rdns, srcHost string, fail bool) {
		rdnsFut := future.New()
		var ptr []string
		if rdns != "" {
			rdnsFut.Set(rdns, nil)
			ptr = []string{rdns}
		} else {
			rdnsFut.Set(nil, nil)
		}

		res := requireMatchingRDNS(check.StatelessCheckContext{
			Resolver: &mockdns.Resolver{
				Zones: map[string]mockdns.Zone{
					"4.3.2.1.in-addr.arpa.": {
						PTR: ptr,
					},
				},
			},
			MsgMeta: &module.MsgMetadata{
				Conn: &module.ConnState{
					ConnectionState: smtp.ConnectionState{
						RemoteAddr: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 55555},
						Hostname:   srcHost,
					},
					RDNSName: rdnsFut,
				},
			},
			Logger: testutils.Logger(t, "require_matching_rdns"),
		})

		actualFail := res.Reason != nil
		if fail && !actualFail {
			t.Errorf("%v, %s: expected failure but check succeeded", rdns, srcHost)
		}
		if !fail && actualFail {
			t.Errorf("%v, %s: unexpected failure", rdns, srcHost)
		}
	}

	test("", "example.org", true)
	test("example.org", "[1.2.3.4]", true)
	test("example.org", "[IPv6:beef::1]", true)
	test("example.org", "example.org", false)
	test("example.org.", "example.org", false)
	test("example.org", "example.org.", false)
	test("example.org.", "example.org.", false)
	test("example.com.", "example.org.", true)
}

func TestRequireMXRecord(t *testing.T) {
	test := func(mailFrom, mxDomain string, mx []net.MX, fail bool) {
		res := requireMXRecord(check.StatelessCheckContext{
			Resolver: &mockdns.Resolver{
				Zones: map[string]mockdns.Zone{
					mxDomain + ".": {
						MX: mx,
					},
				},
			},
			MsgMeta: &module.MsgMetadata{
				Conn: &module.ConnState{
					ConnectionState: smtp.ConnectionState{
						RemoteAddr: &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 55555},
					},
				},
			},
			Logger: testutils.Logger(t, "require_mx_record"),
		}, mailFrom)

		actualFail := res.Reason != nil
		if fail && !actualFail {
			t.Errorf("%v, %v: expected failure but check succeeded", mailFrom, mx)
		}
		if !fail && actualFail {
			t.Errorf("%v, %v: unexpected failure", mailFrom, mx)
		}
	}

	test("foo@example.org", "example.org", nil, true)
	test("foo@example.com", "", nil, true) // NXDOMAIN
	test("foo@[1.2.3.4]", "", nil, true)
	test("[IPv6:beef::1]", "", nil, true)
	test("[IPv6:beef::1]", "", nil, true)
	test("foo@example.org", "example.org", []net.MX{{Host: "a.com"}}, false)
	test("foo@", "", nil, true)
	test("", "", nil, false) // Permit <> for bounces.
	test("foo@example.org", "example.org", []net.MX{{Host: "."}}, true)
}
