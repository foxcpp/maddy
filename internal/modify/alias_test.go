package modify

import (
	"context"
	"testing"
)

type mockTable struct {
	db map[string]string
}

func (m mockTable) Lookup(a string) (string, bool, error) {
	b, ok := m.db[a]
	return b, ok, nil
}

func TestRewriteRcpt(t *testing.T) {
	test := func(addr, expected string, aliases map[string]string) {
		t.Helper()

		s := state{m: &Modifier{
			table: mockTable{db: aliases},
		}}

		actual, err := s.RewriteRcpt(context.Background(), addr)
		if err != nil {
			t.Fatal(err)
		}

		if actual != expected {
			t.Errorf("want %s, got %s", expected, actual)
		}
	}

	test("test@example.org", "test@example.org", nil)
	test("postmaster", "postmaster", nil)
	test("test@example.com", "test@example.org",
		map[string]string{"test@example.com": "test@example.org"})
	test(`"\"test @ test\""@example.com`, "test@example.org",
		map[string]string{`"\"test @ test\""@example.com`: "test@example.org"})
	test(`test@example.com`, `"\"test @ test\""@example.org`,
		map[string]string{`test@example.com`: `"\"test @ test\""@example.org`})
	test(`"\"test @ test\""@example.com`, `"\"b @ b\""@example.com`,
		map[string]string{`"\"test @ test\""`: `"\"b @ b\""`})
	test("TeSt@eXAMple.com", "test@example.org",
		map[string]string{"test@example.com": "test@example.org"})
	test("test@example.com", "test2@example.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test2@example.org",
		map[string]string{"test": "test2@example.org"})
	test("postmaster", "test2@example.org",
		map[string]string{"postmaster": "test2@example.org"})
	test("TeSt@examPLE.com", "test2@example.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test3@example.com",
		map[string]string{
			"test@example.com": "test3@example.com",
			"test":             "test2",
		})
	test("rcpt@E\u0301.example.com", "rcpt@foo.example.com",
		map[string]string{
			"rcpt@\u00E9.example.com": "rcpt@foo.example.com",
		})
	test("E\u0301@foo.example.com", "rcpt@foo.example.com",
		map[string]string{
			"\u00E9@foo.example.com": "rcpt@foo.example.com",
		})
}
