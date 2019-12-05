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
	t.Run("plain string", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, "test@example.org", "test2@example.org")
		val, err := rewriter(r, "test@example.org")
		if err != nil {
			t.Fatal(err)
		}
		if val != "test2@example.org" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("plain string (case insensitive)", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, "test@EXAmple.org", "test2@example.org")
		val, err := rewriter(r, "teST@exaMPLe.oRg")
		if err != nil {
			t.Fatal(err)
		}
		if val != "test2@example.org" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("regexp", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, `/test@example\.org/`, "test2@example.org")
		val, err := rewriter(r, "test@example.org")
		if err != nil {
			t.Fatal(err)
		}
		if val != "test2@example.org" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("regexp (case insensitive)", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, `/test@EXAmple\.org/`, "test2@example.org")
		val, err := rewriter(r, "teST@exaMPLe.oRg")
		if err != nil {
			t.Fatal(err)
		}
		if val != "test2@example.org" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("regexp (incomplete match)", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, `/example/`, "test2@example.org")
		val, err := rewriter(r, "teST@exaMPLe.oRg")
		if err != nil {
			t.Fatal(err)
		}
		if val != "teST@exaMPLe.oRg" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("regexp (result expansion)", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, `/(.+)@example\.org/`, "$1@example.com")
		val, err := rewriter(r, "test@example.org")
		if err != nil {
			t.Fatal(err)
		}
		if val != "test@example.com" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
	t.Run("regexp (result expansion preserves case)", func(t *testing.T) {
		r := replaceAddrFromArgs(t, modName, `/(.+)@example\.org/`, "$1@example.com")
		val, err := rewriter(r, "teST@example.org")
		if err != nil {
			t.Fatal(err)
		}
		if val != "teST@example.com" {
			t.Fatalf("wrong result: %s != %s", val, "test2@example.org")
		}
	})
}

func TestReplaceAddr_RewriteSender(t *testing.T) {
	testReplaceAddr(t, "replace_sender", (*replaceAddr).RewriteSender)
}

func TestReplaceAddr_RewriteRcpt(t *testing.T) {
	testReplaceAddr(t, "replace_rcpt", (*replaceAddr).RewriteRcpt)
}
