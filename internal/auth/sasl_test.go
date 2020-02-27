package auth

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/testutils"
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

func TestCreateSASL(t *testing.T) {
	a := SASLAuth{
		Log: testutils.Logger(t, "saslauth"),
		Plain: []module.PlainAuth{
			&mockAuth{
				db: map[string][]string{
					"user1": []string{"user1a", "user1b"},
				},
			},
		},
	}

	t.Run("XWHATEVER", func(t *testing.T) {
		srv := a.CreateSASL("XWHATEVER", &net.TCPAddr{}, func([]string) error { return nil })
		_, _, err := srv.Next([]byte(""))
		if err == nil {
			t.Error("No error for XWHATEVER use")
		}
	})

	t.Run("PLAIN", func(t *testing.T) {
		var ids []string
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(passed []string) error {
			ids = passed
			return nil
		})

		_, _, err := srv.Next([]byte("\x00user1\x00aa"))
		if err != nil {
			t.Error("Unexpected error:", err)
		}
		if !reflect.DeepEqual(ids, []string{"user1a", "user1b"}) {
			t.Error("Wrong auth. identities passed to callback:", ids)
		}
	})

	t.Run("PLAIN with autorization identity", func(t *testing.T) {
		var ids []string
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(passed []string) error {
			ids = passed
			return nil
		})

		_, _, err := srv.Next([]byte("user1a\x00user1\x00aa"))
		if err != nil {
			t.Error("Unexpected error:", err)
		}
		if !reflect.DeepEqual(ids, []string{"user1a"}) {
			t.Error("Wrong auth. identities passed to callback:", ids)
		}
	})

	t.Run("PLAIN with wrong authorization identity", func(t *testing.T) {
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(passed []string) error {
			return nil
		})

		_, _, err := srv.Next([]byte("user1c\x00user1\x00aa"))
		if err == nil {
			t.Error("Next should fail")
		}
	})

	t.Run("PLAIN with wrong authentication identity", func(t *testing.T) {
		srv := a.CreateSASL("PLAIN", &net.TCPAddr{}, func(passed []string) error {
			return nil
		})

		_, _, err := srv.Next([]byte("\x00user2\x00aa"))
		if err == nil {
			t.Error("Next should fail")
		}
	})
}
