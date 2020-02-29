package pass_table

import (
	"testing"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestAuth_AuthPlain(t *testing.T) {
	addSHA256()

	mod, err := New("pass_table", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = mod.Init(config.NewMap(nil, &config.Node{
		Children: []config.Node{
			{
				// We replace it later with our mock.
				Name: "table",
				Args: []string{"dummy"},
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	a := mod.(*Auth)
	a.table = testutils.Table{
		M: map[string]string{
			"foxcpp":       "sha256:U0FMVA==:8PDRAgaUqaLSk34WpYniXjaBgGM93Lc6iF4pw2slthw=",
			"not-foxcpp":   "bcrypt:$2y$10$4tEJtJ6dApmhETg8tJ4WHOeMtmYXQwmHDKIyfg09Bw1F/smhLjlaa",
			"not-foxcpp-2": "argon2:1:8:1:U0FBQUFBTFQ=:KHUshl3DcpHR3AoVd28ZeBGmZ1Fj1gwJgNn98Ia8DAvGHqI0BvFOMJPxtaAfO8F+qomm2O3h0P0yV50QGwXI/Q==",
		},
	}

	check := func(user, pass string, ok bool) {
		t.Helper()

		err := a.AuthPlain(user, pass)
		if (err == nil) != ok {
			t.Errorf("ok=%v, err: %v", ok, err)
		}
	}

	check("foxcpp", "password", true)
	check("foxcpp", "different-password", false)
	check("not-foxcpp", "password", true)
	check("not-foxcpp", "different-password", false)
	check("not-foxcpp-2", "password", true)
}
