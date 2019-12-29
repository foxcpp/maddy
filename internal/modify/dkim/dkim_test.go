package dkim

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func newTestModifier(t *testing.T, dir, keyAlgo string) *Modifier {
	mod, err := New("", "test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*Modifier)
	m.log = testutils.Logger(t, m.Name())

	err = m.Init(config.NewMap(nil, &config.Node{
		Children: []config.Node{
			{
				Name: "domain",
				Args: []string{"maddy.test"},
			},
			{
				Name: "selector",
				Args: []string{"default"},
			},
			{
				Name: "key_path",
				Args: []string{filepath.Join(dir, "testkey.key")},
			},
			{
				Name: "require_sender_match",
				Args: []string{"off"},
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

func signTestMsg(t *testing.T, m *Modifier) (textproto.Header, []byte) {
	state, err := m.ModStateForMsg(context.Background(), &module.MsgMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	testHdr := textproto.Header{}
	testHdr.Add("From", "<hello@hello>")
	testHdr.Add("Subject", "heya")
	testHdr.Add("To", "<heya@heya>")
	body := []byte("hello there\r\n")

	err = state.RewriteBody(context.Background(), &testHdr, buffer.MemoryBuffer{Slice: body})
	if err != nil {
		t.Fatal(err)
	}

	return testHdr, body
}

func verifyTestMsg(t *testing.T, dnsPath string, hdr textproto.Header, body []byte) {
	dnsRecord, err := ioutil.ReadFile(dnsPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("DNS record:", string(dnsRecord))

	zones := map[string]mockdns.Zone{
		"default._domainkey.maddy.test.": {
			TXT: []string{string(dnsRecord)},
		},
	}
	t.Log(string(dnsRecord))
	// dkim.Verify does not allow to override its lookup routine, so we have to
	// hjack the global resolver object.
	srv, err := mockdns.NewServer(zones)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	srv.PatchNet(net.DefaultResolver)
	defer mockdns.UnpatchNet(net.DefaultResolver)

	var fullBody bytes.Buffer
	if err := textproto.WriteHeader(&fullBody, hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := fullBody.Write(body); err != nil {
		t.Fatal(err)
	}

	v, err := dkim.Verify(bytes.NewReader(fullBody.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 {
		t.Fatal("Expected exactly one verification")
	}
	if v[0].Err != nil {
		t.Fatal("Verification error:", v[0].Err)
	}
}

func TestGenerateSignVerify(t *testing.T) {
	// This test verifies whether a freshly generated key can be used for
	// signing and verification.
	//
	// It is a kind of "integration" test for DKIM modifier, as it tests
	// whether everything works correctly together.

	test := func(keyAlgo string, headerCanon, bodyCanon dkim.Canonicalization, reload bool) {
		t.Helper()

		dir, err := ioutil.TempDir("", "maddy-tests-dkim-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		// Create the key.
		m := newTestModifier(t, dir, keyAlgo)
		if reload {
			m = newTestModifier(t, dir, keyAlgo)
		}

		testHdr, body := signTestMsg(t, m)
		verifyTestMsg(t, filepath.Join(dir, "testkey.dns"), testHdr, body)
	}

	for _, algo := range [2]string{"rsa2048", "ed25519"} {
		for _, hdrCanon := range [2]dkim.Canonicalization{dkim.CanonicalizationSimple, dkim.CanonicalizationRelaxed} {
			for _, bodyCanon := range [2]dkim.Canonicalization{dkim.CanonicalizationSimple, dkim.CanonicalizationRelaxed} {
				test(algo, hdrCanon, bodyCanon, false)
				test(algo, hdrCanon, bodyCanon, true)
			}
		}
	}
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

func TestShouldSign(t *testing.T) {
	type testCase struct {
		MatchMethods []string

		KeyDomain    string
		AuthIdentity string
		MAILFROM     string
		From         string
		EAI          bool

		ShouldSign bool
		ExpectedId string
	}

	test := func(i int, c testCase) {
		h := textproto.Header{}
		h.Add("From", c.From)

		m := Modifier{
			domain:      c.KeyDomain,
			senderMatch: make(map[string]struct{}, len(c.MatchMethods)),
			log:         testutils.Logger(t, "sign_dkim"),
		}
		for _, method := range c.MatchMethods {
			m.senderMatch[method] = struct{}{}
		}

		id, ok := m.shouldSign(c.EAI, strconv.Itoa(i), &h, c.MAILFROM, c.AuthIdentity)
		if ok != c.ShouldSign {
			t.Errorf("%d %+v: expected ShouldSign=%v, got %v", i, c, c.ShouldSign, ok)
		}
		if id != c.ExpectedId {
			t.Errorf("%d %+v: expected id=%v, got %v", i, c, c.ExpectedId, id)
		}
	}

	cases := []testCase{
		{
			MatchMethods: []string{"off"},

			KeyDomain: "example.org",
			From:      "foo@example.org",

			ShouldSign: true,
			ExpectedId: "@example.org",
		},
		{
			MatchMethods: []string{"off"},

			KeyDomain: "ñaca.com",
			From:      "foo@ñaca.com",

			ShouldSign: true,
			ExpectedId: "@xn--aca-6ma.com",
		},
		{
			MatchMethods: []string{"off"},

			KeyDomain: "ñaca.com",
			From:      "foo@ñaca.com",
			EAI:       true,

			ShouldSign: true,
			ExpectedId: "@ñaca.com",
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "example.org",
			From:      "foo@example.org",
			MAILFROM:  "baz@example.org",

			ShouldSign: false,
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "example.com",
			From:      "foo@example.org",
			MAILFROM:  "foo@example.org",

			ShouldSign: false,
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "example.org",
			From:      "foo@example.org",
			MAILFROM:  "foo@example.org",

			ShouldSign: true,
			ExpectedId: "foo@example.org",
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "ñaca.com",
			From:      "foo@ñaca.com",
			MAILFROM:  "foo@ñaca.com",

			ShouldSign: true,
			ExpectedId: "foo@xn--aca-6ma.com",
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "example.com",
			From:      "ñ@example.com",
			MAILFROM:  "ñ@example.com",

			ShouldSign: true,
			ExpectedId: "@example.com",
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "example.com",
			From:      "ñ@example.com",
			MAILFROM:  "ñ@example.com",
			EAI:       true,

			ShouldSign: true,
			ExpectedId: "ñ@example.com",
		},
		{
			MatchMethods: []string{"envelope"},

			KeyDomain: "ñaca.com",
			From:      "foo@ñaca.com",
			MAILFROM:  "foo@ñaca.com",
			EAI:       true,

			ShouldSign: true,
			ExpectedId: "foo@ñaca.com",
		},
		{
			MatchMethods: []string{"envelope", "auth"},

			KeyDomain:    "example.org",
			From:         "foo@example.org",
			MAILFROM:     "foo@example.org",
			AuthIdentity: "baz",

			ShouldSign: false,
		},
		{
			MatchMethods: []string{"auth"},

			KeyDomain:    "example.org",
			From:         "foo@example.org",
			MAILFROM:     "foo@example.com",
			AuthIdentity: "foo",

			ShouldSign: true,
			ExpectedId: "foo@example.org",
		},
		{
			MatchMethods: []string{"envelope", "auth"},

			KeyDomain:    "example.com",
			From:         "foo@example.com",
			MAILFROM:     "foo@example.com",
			AuthIdentity: "foo@example.org",

			ShouldSign: false,
		},
	}
	for i, case_ := range cases {
		test(i, case_)
	}
}
