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

package dmarc

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/foxcpp/go-mockdns"
)

func TestDMARC(t *testing.T) {
	test := func(zones map[string]mockdns.Zone, hdr string, authres []authres.Result, policyApplied Policy, dmarcRes authres.ResultValue) {
		t.Helper()
		v := NewVerifier(&mockdns.Resolver{Zones: zones})
		defer v.Close()

		hdrParsed, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(hdr)))
		if err != nil {
			panic(err)
		}
		v.FetchRecord(context.Background(), hdrParsed)
		evalRes, policy := v.Apply(authres)

		if policy != policyApplied {
			t.Errorf("expected applied policy to be '%v', got '%v'", policyApplied, policy)
		}
		if evalRes.Authres.Value != dmarcRes {
			t.Errorf("expected DMARC result to be '%v', got '%v'", dmarcRes, evalRes.Authres.Value)
		}
	}

	// No policy => DMARC 'none'
	test(map[string]mockdns.Zone{}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultNone)

	// Policy present & identifiers align => DMARC 'pass'
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPass)

	// No SPF check run => DMARC 'none', no action taken
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"v=DMARC1; p=reject"},
		},
	}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
	}, PolicyNone, authres.ResultNone)

	// No DKIM check run => DMARC 'none', no action taken
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"v=DMARC1; p=reject"},
		},
	}, "From: hello@example.org\r\n\r\n", []authres.Result{
		&authres.SPFResult{Value: authres.ResultPass, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultNone)

	// Check org. domain and from domain, prefer from domain.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPass)
	test(map[string]mockdns.Zone{
		"_dmarc.sub.example.org.": {
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPass)
	test(map[string]mockdns.Zone{
		"_dmarc.sub.example.org.": {
			TXT: []string{"v=DMARC1; p=none"},
		},
		"_dmarc.example.org.": {
			TXT: []string{"v=malformed"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPass)

	// Non-DMARC records are ignored.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"ignore", "v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPass)

	// Multiple policies => no policy.
	// https://tools.ietf.org/html/rfc7489#section-6.6.3
	test(map[string]mockdns.Zone{
		"_dmarc.example.org.": {
			TXT: []string{"v=DMARC1; p=reject", "v=DMARC1; p=none"},
		},
	}, "From: hello@sub.example.org\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultNone)

	// Malformed policy => no policy
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			TXT: []string{"v=aaaa"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultNone)

	// Policy fetch error => DMARC 'permerror' but the message
	// is accepted.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			Err: errors.New("the dns server is going insane"),
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultPermError)

	// Policy fetch error => DMARC 'temperror' but the message
	// is accepted ("fail closed")
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			Err: &net.DNSError{
				Err:         "the dns server is going insane, temporary",
				IsTemporary: true,
			},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyReject, authres.ResultTempError)

	// Misaligned From vs DKIM => DMARC 'fail'.
	// Side note: More comprehensive tests for alignment evaluation
	// can be found in check/dmarc/evaluate_test.go. This test merely checks
	// that the correct action is taken based on the policy.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			TXT: []string{"v=DMARC1; p=none"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultFail)

	// Misaligned From vs DKIM => DMARC 'fail', policy says to reject
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			TXT: []string{"v=DMARC1; p=reject"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyReject, authres.ResultFail)

	// Misaligned From vs DKIM => DMARC 'fail'
	// Subdomain policy requests no action, main domain policy says to reject.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			TXT: []string{"v=DMARC1; sp=none; p=reject"},
		},
	}, "From: hello@sub.example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyNone, authres.ResultFail)

	// Misaligned From vs DKIM => DMARC 'fail', policy says to quarantine.
	test(map[string]mockdns.Zone{
		"_dmarc.example.com.": {
			TXT: []string{"v=DMARC1; p=quarantine"},
		},
	}, "From: hello@example.com\r\n\r\n", []authres.Result{
		&authres.DKIMResult{Value: authres.ResultPass, Domain: "example.org"},
		&authres.SPFResult{Value: authres.ResultNone, From: "example.org", Helo: "mx.example.org"},
	}, PolicyQuarantine, authres.ResultFail)
}
