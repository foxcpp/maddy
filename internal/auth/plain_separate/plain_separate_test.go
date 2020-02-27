package plain_separate

import (
	"errors"
	"reflect"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/internal/module"
)

type mockAuth struct {
	db map[string][]string
}

func (mockAuth) SASLMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

func (m mockAuth) AuthPlain(username, _ string) ([]string, error) {
	ids, ok := m.db[username]
	if !ok {
		return nil, errors.New("invalid creds")
	}
	return ids, nil
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
				db: map[string][]string{
					"user1": []string{"user1a", "user1b"},
				},
			},
		},
	}

	ids, err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(ids, []string{"user1a", "user1b"}) {
		t.Fatal("Wrong ids returned:", ids)
	}
}

func TestPlainSplit_NoUser_MultiPass(t *testing.T) {
	a := Auth{
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string][]string{
					"user2": []string{"user2a", "user2b"},
				},
			},
			mockAuth{
				db: map[string][]string{
					"user1": []string{"user1a", "user1b"},
				},
			},
		},
	}

	ids, err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(ids, []string{"user1a", "user1b"}) {
		t.Fatal("Wrong ids returned:", ids)
	}
}

func TestPlainSplit_UserPass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"user1": "user2",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string][]string{
					"user2": []string{"user2a", "user2b"},
				},
			},
			mockAuth{
				db: map[string][]string{
					"user1": []string{"user1a", "user1b"},
				},
			},
		},
	}

	ids, err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(ids, []string{"user2"}) {
		t.Fatal("Wrong ids returned:", ids)
	}
}

func TestPlainSplit_MultiUser_Pass(t *testing.T) {
	a := Auth{
		userTbls: []module.Table{
			mockTable{
				db: map[string]string{
					"userWH": "user1",
				},
			},
			mockTable{
				db: map[string]string{
					"user1": "user2",
				},
			},
		},
		passwd: []module.PlainAuth{
			mockAuth{
				db: map[string][]string{
					"user2": []string{"user2a", "user2b"},
				},
			},
			mockAuth{
				db: map[string][]string{
					"user1": []string{"user1a", "user1b"},
				},
			},
		},
	}

	ids, err := a.AuthPlain("user1", "aaa")
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if !reflect.DeepEqual(ids, []string{"user2"}) {
		t.Fatal("Wrong ids returned:", ids)
	}
}
