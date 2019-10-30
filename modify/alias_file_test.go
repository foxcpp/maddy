package modify

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestReadFile(t *testing.T) {
	test := func(file string, expected map[string]string) {
		t.Helper()

		f, err := ioutil.TempFile("", "maddy-tests-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(file); err != nil {
			t.Fatal(err)
		}

		actual := map[string]string{}
		err = readFile(f.Name(), actual)
		if expected == nil {
			if err == nil {
				t.Errorf("expected failure, got %+v", actual)
			}
			return
		}
		if err != nil {
			t.Errorf("unexpected failure: %v", err)
			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("wrong results\n want %+v\n got %+v", expected, actual)
		}
	}

	test("a: b", map[string]string{"a": "b"})
	test("a: b, c", nil)
	test(": b", nil)
	test(":", nil)
	test("aaa", nil)
	test(": b", nil)
	test("     testing@example.com   :  arbitrary-whitespace@example.org   ",
		map[string]string{"testing@example.com": "arbitrary-whitespace@example.org"})
	test(`# skip comments
a: b`, map[string]string{"a": "b"})
	test(`# and empty lines

a: b`, map[string]string{"a": "b"})
	test("# with whitespace too\n    \na: b", map[string]string{"a": "b"})
	test("A: b", map[string]string{"a": "b"})
	test("CaSe: FoLdInG@ExAmPle.OrG", map[string]string{"case": "folding@example.org"})

	// In case of postmaster handling we are not sure whether this is a <postmaster>
	// or <postmaster@domain> since we try both.
	// Given 'postmaster: b', <postmaster> will be rewritten to <b>
	// but <postmaster@domain> will be rewritten to <b@domain>.
	// As <b> is not a valid address, this can cause problems. Hence we reject
	// this case due to ambiguity and require fully qualified address as a
	// replacement for 'postmaster'.
	test("postmaster: b", nil)
	test("postmaster: b@example.org", map[string]string{"postmaster": "b@example.org"})
}

func TestRewriteRcpt(t *testing.T) {
	test := func(addr, expected string, aliases map[string]string) {
		t.Helper()

		s := state{m: &Modifier{
			aliases: aliases,
		}}

		actual, err := s.RewriteRcpt(addr)
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
	test("TeSt@eXAMple.com", "test@example.org",
		map[string]string{"test@example.com": "test@example.org"})
	test("test@example.com", "test2@example.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test2@example.org",
		map[string]string{"test": "test2@example.org"})
	test("postmaster", "test2@example.org",
		map[string]string{"postmaster": "test2@example.org"})
	test("TeSt@examPLE.com", "test2@examPLE.com",
		map[string]string{"test": "test2"})
	test("test@example.com", "test3@example.com",
		map[string]string{
			"test@example.com": "test3@example.com",
			"test":             "test2",
		})

	// This case merely documents current behavior, I am not sure
	// whether this is the right behavior or it should hard-fail
	// in this case.
	test("postmaster", "test2",
		map[string]string{"postmaster": "test2"})
}
