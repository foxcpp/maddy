package modify

import (
	"testing"

	"github.com/foxcpp/maddy/internal/config"
)

func replaceAddrFromArgs(t *testing.T, modName, from, to string) *replaceAddr {
	r, err := NewReplaceAddr(modName, "", nil, []string{from, to})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Init(&config.Map{Block: &config.Node{}}); err != nil {
		t.Fatal(err)
	}

	return r.(*replaceAddr)
}

func testReplaceAddr(t *testing.T, modName string, rewriter func(*replaceAddr, string) (string, error)) {
	test := func(from, to string, input, expectedOutput string) {
		t.Helper()

		r := replaceAddrFromArgs(t, modName, from, to)
		output, err := rewriter(r, input)
		if err != nil {
			t.Fatal(err)
		}
		if output != expectedOutput {
			t.Fatalf("wrong result: %s != %s", output, expectedOutput)
		}
	}

	test("test@example.org", "test2@example.org", "test@example.org", "test2@example.org")
	test("test@EXAmple.org", "test2@example.org", "teST@exaMPLe.org", "test2@example.org")
	test(`/test@example\.org/`, "test2@example.org", "test@example.org", "test2@example.org")
	test(`/test@EXAmple\.org/`, "test2@example.org", "teST@exaMPLe.org", "test2@example.org")
	test(`/example/`, "test2@example.org", "teST@exaMPLe.org", "teST@exaMPLe.org")
	test(`/(.+)@example\.org/`, "$1@example.com", "test@example.org", "test@example.com")
	test(`/(.+)@example\.org/`, "$1@example.com", "teST@example.org", "teST@example.com")
	test(`/(.+)@example\.org/`, "$1@example.com", "teST@example.org", "teST@example.com")
	test("rcpt@\u00E9.example.com", "rcpt@foo.example.com", "rcpt@E\u0301.example.com", "rcpt@foo.example.com")
	test(`/rcpt@é\.example\.com/`, "rcpt@foo.example.com", "rcpt@E\u0301.example.com", "rcpt@foo.example.com")
	test("rcpt@E\u0301.example.com", "rcpt@foo.example.com", "rcpt@\u00E9.example.com", "rcpt@foo.example.com")
}

func TestReplaceAddr_RewriteSender(t *testing.T) {
	testReplaceAddr(t, "replace_sender", (*replaceAddr).RewriteSender)
}

func TestReplaceAddr_RewriteRcpt(t *testing.T) {
	testReplaceAddr(t, "replace_rcpt", (*replaceAddr).RewriteRcpt)
}
