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

package dkim

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func newTestModifier(t *testing.T, dir, keyAlgo string, domains []string) *Modifier {
	mod, err := New("", "test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*Modifier)
	m.log = testutils.Logger(t, m.Name())

	err = m.Init(config.NewMap(nil, config.Node{
		Children: []config.Node{
			{
				Name: "domains",
				Args: domains,
			},
			{
				Name: "selector",
				Args: []string{"default"},
			},
			{
				Name: "key_path",
				Args: []string{filepath.Join(dir, "{domain}.key")},
			},
			{
				Name: "newkey_algo",
				Args: []string{keyAlgo},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	return m
}

func signTestMsg(t *testing.T, m *Modifier, envelopeFrom string) (textproto.Header, []byte) {
	t.Helper()

	state, err := m.ModStateForMsg(context.Background(), &module.MsgMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	testHdr := textproto.Header{}
	testHdr.Add("From", "<hello@hello>")
	testHdr.Add("Subject", "heya")
	testHdr.Add("To", "<heya@heya>")
	body := []byte("hello there\r\n")

	// modify.dkim expects RewriteSender to be called to get envelope sender
	//  (see module.Modifier docs)

	// RewriteSender does not fail for modify.dkim. It just sets envelopeFrom.
	if _, err := state.RewriteSender(context.Background(), envelopeFrom); err != nil {
		panic(err)
	}
	err = state.RewriteBody(context.Background(), &testHdr, buffer.MemoryBuffer{Slice: body})
	if err != nil {
		t.Fatal(err)
	}

	return testHdr, body
}

func verifyTestMsg(t *testing.T, keysPath string, expectedDomains []string, hdr textproto.Header, body []byte) {
	t.Helper()

	domainsMap := make(map[string]bool)
	zones := map[string]mockdns.Zone{}
	for _, domain := range expectedDomains {
		dnsRecord, err := ioutil.ReadFile(filepath.Join(keysPath, domain+".dns"))
		if err != nil {
			t.Fatal(err)
		}

		t.Log("DNS record:", string(dnsRecord))
		zones["default._domainkey."+domain+"."] = mockdns.Zone{TXT: []string{string(dnsRecord)}}
		domainsMap[domain] = false
	}

	var fullBody bytes.Buffer
	if err := textproto.WriteHeader(&fullBody, hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := fullBody.Write(body); err != nil {
		t.Fatal(err)
	}

	resolver := &mockdns.Resolver{Zones: zones}
	verifs, err := dkim.VerifyWithOptions(bytes.NewReader(fullBody.Bytes()), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			return resolver.LookupTXT(context.Background(), domain)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range verifs {
		if v.Err != nil {
			t.Errorf("Verification error for %s: %v", v.Domain, v.Err)
		}
		if _, ok := domainsMap[v.Domain]; !ok {
			t.Errorf("Unexpected verification for domain %s", v.Domain)
		}

		domainsMap[v.Domain] = true
	}
	for domain, ok := range domainsMap {
		if !ok {
			t.Errorf("Missing verification for domain %s", domain)
		}
	}
}

func TestGenerateSignVerify(t *testing.T) {
	// This test verifies whether a freshly generated key can be used for
	// signing and verification.
	//
	// It is a kind of "integration" test for DKIM modifier, as it tests
	// whether everything works correctly together.
	//
	// Additionally it also tests whether key selection works correctly.

	test := func(domains []string, envelopeFrom string, expectDomain []string, keyAlgo string, headerCanon, bodyCanon dkim.Canonicalization, reload bool) {
		t.Helper()

		dir, err := ioutil.TempDir("", "maddy-tests-dkim-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		m := newTestModifier(t, dir, keyAlgo, domains)
		m.bodyCanon = bodyCanon
		m.headerCanon = headerCanon
		if reload {
			m = newTestModifier(t, dir, keyAlgo, domains)
		}

		testHdr, body := signTestMsg(t, m, envelopeFrom)
		verifyTestMsg(t, dir, expectDomain, testHdr, body)
	}

	for _, algo := range [2]string{"rsa2048", "ed25519"} {
		for _, hdrCanon := range [2]dkim.Canonicalization{dkim.CanonicalizationSimple, dkim.CanonicalizationRelaxed} {
			for _, bodyCanon := range [2]dkim.Canonicalization{dkim.CanonicalizationSimple, dkim.CanonicalizationRelaxed} {
				test([]string{"maddy.test"}, "test@maddy.test", []string{"maddy.test"}, algo, hdrCanon, bodyCanon, false)
				test([]string{"maddy.test"}, "test@maddy.test", []string{"maddy.test"}, algo, hdrCanon, bodyCanon, true)
			}
		}
	}

	// Key selection tests
	test(
		[]string{"maddy.test"}, // Generated keys.
		"test@maddy.test",      // Envelope sender.
		[]string{"maddy.test"}, // Expected signature domains.
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"maddy.test"},
		"test@unrelated.maddy.test",
		[]string{},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"maddy.test", "related.maddy.test"},
		"test@related.maddy.test",
		[]string{"related.maddy.test"},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"fallback.maddy.test", "maddy.test"},
		"postmaster",
		[]string{"fallback.maddy.test"},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"fallback.maddy.test", "maddy.test"},
		"",
		[]string{"fallback.maddy.test"},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"another.maddy.test", "another.maddy.test", "maddy.test"},
		"test@another.maddy.test",
		[]string{"another.maddy.test"},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
	test(
		[]string{"another.maddy.test", "another.maddy.test", "maddy.test"},
		"",
		[]string{"another.maddy.test"},
		"ed25519", dkim.CanonicalizationRelaxed, dkim.CanonicalizationRelaxed, false)
}

func TestFieldsToSign(t *testing.T) {
	h := textproto.Header{}
	h.Add("A", "1")
	h.Add("c", "2")
	h.Add("C", "3")
	h.Add("a", "4")
	h.Add("b", "5")
	h.Add("unrelated", "6")

	m := Modifier{
		oversignHeader: []string{"A", "B"},
		signHeader:     []string{"C"},
	}
	fields := m.fieldsToSign(&h)
	sort.Strings(fields)
	expected := []string{"A", "A", "A", "B", "B", "C", "C"}

	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("incorrect set of fields to sign\nwant: %v\ngot:  %v", expected, fields)
	}
}
