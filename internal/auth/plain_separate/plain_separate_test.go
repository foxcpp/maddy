package plain_separate

import (
	"errors"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/internal/module"
)

type mockAuth struct {
	db map[string]bool
}

func (mockAuth) SASLMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

func (m mockAuth) AuthPlain(username, _ string) error {
	ok := m.db[username]
	if !ok {
		return errors.New("invalid creds")
	}
	return nil
}

type mockTable struct {
	db map[string]string
}

func (m mockTable) Lookup(a string) (string, bool, error) {
	b, ok := m.db[a]
	return b, ok, nil
}

func TestPlainSplit_NoUser(t *testing.T) {
	a := Auth{
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_NoUser_MultiPass(t *testing.T) {
	a := Auth{
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_UserPass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"user1": "",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}

func TestPlainSplit_MultiUser_Pass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"userWH": "",
				},
			},
			mockTable{
				db: map[string]string{
					"user1": "",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string]bool{
					"user2": true,
				},
			},
			mockAuth{
				db: map[string]bool{
					"user1": true,
				},
			},
		},
	}

	err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
}
