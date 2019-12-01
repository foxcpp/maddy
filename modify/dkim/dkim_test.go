package dkim

import (
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/testutils"
)

// TODO: Add tests that check actual signing once
// we have LookupTXT hook for go-msgauth/dkim.

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
