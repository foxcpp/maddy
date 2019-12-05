package auth

import (
	"fmt"
	"testing"
)

func TestCheckDomainAuth(t *testing.T) {
	cases := []struct {
		rawUsername string

		perDomain      bool
		allowedDomains []string

		loginName string
	}{
		{
			rawUsername: "username",
			loginName:   "username",
		},
		{
			rawUsername:    "username",
			allowedDomains: []string{"example.org"},
			loginName:      "username",
		},
		{
			rawUsername:    "username@example.org",
			allowedDomains: []string{"example.org"},
			loginName:      "username",
		},
		{
			rawUsername:    "username@example.com",
			allowedDomains: []string{"example.org"},
		},
		{
			rawUsername:    "username",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
		},
		{
			rawUsername:    "username@example.com",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
		},
		{
			rawUsername:    "username@EXAMPLE.Org",
			allowedDomains: []string{"exaMPle.org"},
			perDomain:      true,
			loginName:      "username@EXAMPLE.Org",
		},
		{
			rawUsername:    "username@example.org",
			allowedDomains: []string{"example.org"},
			perDomain:      true,
			loginName:      "username@example.org",
		},
	}

	for _, case_ := range cases {
		case_ := case_
		t.Run(fmt.Sprintf("%+v", case_), func(t *testing.T) {
			loginName, allowed := CheckDomainAuth(case_.rawUsername, case_.perDomain, case_.allowedDomains)
			if case_.loginName != "" && !allowed {
				t.Fatalf("Unexpected authentication fail")
			}
			if case_.loginName == "" && allowed {
				t.Fatalf("Expected authentication fail, got %s as login name", loginName)
			}

			if loginName != case_.loginName {
				t.Errorf("Incorrect login name, got %s, wanted %s", loginName, case_.loginName)
			}
		})
	}
}
