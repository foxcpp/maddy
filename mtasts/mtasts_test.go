package mtasts

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReadDNSRecord(t *testing.T) {
	cases := []struct {
		value string
		id    string
		fail  bool
	}{
		{
			value: "",
			fail:  true,
		},
		{
			value: "v=STSv1",
			fail:  true,
		},
		{
			value: "id=foo",
			fail:  true,
		},
		{
			value: "unrelated=foo",
			fail:  true,
		},
		{
			value: "syntax error",
			fail:  true,
		},
		{
			value: "v=STSv2;id=foo;include=foo.com",
			fail:  true,
		},
		{
			value: "v=STSv1;    id=foo include=foo.com",
			fail:  true,
		},
		{
			value: "v=STSv1;    id=foo include",
			fail:  true,
		},
		{
			value: "v=STSv1  ;    id=foo",
			id:    "foo",
		},
		{
			value: "v=STSv1  ;    id=foo; unrelated=1",
			id:    "foo",
		},
	}

	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			id, err := readDNSRecord(c.value)
			if c.fail {
				if err == nil {
					t.Errorf("expected failure for %v, but got with id=%v", c.value, id)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected failure for %v: %v", c.value, err)
					return
				}

				if id != c.id {
					t.Errorf("expected id %v, got %v", c.id, id)
				}
			}
		})
	}
}

func TestReadPolicy(t *testing.T) {
	cases := []struct {
		value  string
		policy *Policy
		fail   bool
	}{
		{
			value: `version: STSv2`,
			fail:  true,
		},
		{
			value: `version: STSv1`,
			fail:  true,
		},
		{
			value: `max_age: 8600`,
			fail:  true,
		},
		{
			value: `version: STSv1
max_age: 8600`,
			fail: true,
		},
		{
			value: `version: STSv1
max_age:`,
			fail: true,
		},
		{
			value: `version: STSv1
: 8600`,
			fail: true,
		},
		{
			value: `version: STSv1
mode: invalid_value`,
			fail: true,
		},
		{
			value: `version: STSv1
mode none`,
			fail: true,
		},
		{
			value: `version: STSv1
mode: none`,
			fail: true,
		},
		{
			value: `version: STSv1
max_age: 8600
mode:none`,
			policy: &Policy{
				Mode:   ModeNone,
				MaxAge: 8600,
			},
		},
		{
			value: `version: STSv1
max_age: 8600
mode: enforce`,
			fail: true,
		},
		{
			value: `version: STSv1
max_age: 8600
mode: enforce
mx: mx0.example.org
mx: *.example.org`,
			policy: &Policy{
				Mode:   ModeEnforce,
				MaxAge: 8600,
				MX:     []string{"mx0.example.org", "*.example.org"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			p, err := readPolicy(strings.NewReader(c.value))
			if c.fail {
				if err == nil {
					t.Errorf("expected failure, but got %+v", p)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected failure: %v", err)
					return
				}

				if !reflect.DeepEqual(c.policy, p) {
					t.Log("unexpected read result")
					t.Log("policy:")
					t.Log(c.value)
					t.Logf("expected result: %+v", c.policy)
					t.Logf("actual result: %+v", p)
					t.Fail()
				}
			}
		})
	}
}

func TestPolicyMatch(t *testing.T) {
	cases := []struct {
		mx          string
		validMXs    []string
		shouldMatch bool
	}{
		{
			mx:          "example.org.",
			validMXs:    []string{"example.org"},
			shouldMatch: true,
		},
		{
			mx:          "example.org",
			validMXs:    []string{"example.org"},
			shouldMatch: true,
		},
		{
			mx:          "mx0.example.org",
			validMXs:    []string{"special.example.org", "*.example.org"},
			shouldMatch: true,
		},
		{
			mx:          "without-dots",
			validMXs:    []string{"*.example.org"},
			shouldMatch: false,
		},
		{
			mx:          "special.example.org",
			validMXs:    []string{"special.example.org", "*.example.org"},
			shouldMatch: true,
		},
		{
			mx:          "mx0.special.example.org",
			validMXs:    []string{"special.example.org", "*.example.org"},
			shouldMatch: false,
		},
		{
			mx:          "mx0.special.example.org",
			validMXs:    []string{"*.special.example.org", "*.example.org"},
			shouldMatch: true,
		},
		{
			mx:          "unrelated.org",
			validMXs:    []string{"*.example.org"},
			shouldMatch: false,
		},
		{
			mx:          "ñaca.com",
			validMXs:    []string{"ñaca.com"},
			shouldMatch: true,
		},
		{
			mx:          "Ñaca.com",
			validMXs:    []string{"ñaca.com"},
			shouldMatch: true,
		},
		{
			mx:          "ñaca.com",
			validMXs:    []string{"Ñaca.com"},
			shouldMatch: true,
		},
		{
			mx:          "x.ñaca.com",
			validMXs:    []string{"x.xn--aca-6ma.com"},
			shouldMatch: true,
		},
		{
			mx:          "x.naca.com",
			validMXs:    []string{"x.xn--aca-6ma.com"},
			shouldMatch: false,
		},
		{
			mx:          "example.org",
			validMXs:    []string{"xn-9999999999a.org"},
			shouldMatch: false,
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintln(c.mx, c.validMXs), func(t *testing.T) {
			p := Policy{MX: c.validMXs}

			matched := p.Match(c.mx)
			if c.shouldMatch && !matched {
				t.Error(c.mx, "didn't match", c.validMXs, "but it should")
			}
			if !c.shouldMatch && matched {
				t.Error(c.mx, "matched", c.validMXs, "but it shouldn't")
			}
		})
	}
}
