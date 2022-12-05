/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package table

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestReadFile(t *testing.T) {
	test := func(file string, expected map[string][]string) {
		t.Helper()

		f, err := ioutil.TempFile("", "maddy-tests-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()
		if _, err := f.WriteString(file); err != nil {
			t.Fatal(err)
		}

		actual := map[string][]string{}
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

	test("a: b", map[string][]string{"a": {"b"}})
	test("a@example.org: b@example.com", map[string][]string{"a@example.org": {"b@example.com"}})
	test(`"a @ a"@example.org: b@example.com`, map[string][]string{`"a @ a"@example.org`: {"b@example.com"}})
	test(`a@example.org: "b @ b"@example.com`, map[string][]string{`a@example.org`: {`"b @ b"@example.com`}})
	test(`"a @ a": "b @ b"`, map[string][]string{`"a @ a"`: {`"b @ b"`}})
	test("a: b, c", map[string][]string{"a": {"b", "c"}})
	test("a: b\na: c", map[string][]string{"a": {"b", "c"}})
	test(": b", nil)
	test(":", nil)
	test("aaa", map[string][]string{"aaa": {""}})
	test(": b", nil)
	test("     testing@example.com   :  arbitrary-whitespace@example.org   ",
		map[string][]string{"testing@example.com": {"arbitrary-whitespace@example.org"}})
	test(`# skip comments
a: b`, map[string][]string{"a": {"b"}})
	test(`# and empty lines

a: b`, map[string][]string{"a": {"b"}})
	test("# with whitespace too\n    \na: b", map[string][]string{"a": {"b"}})
	test("a: b\na: c", map[string][]string{"a": {"b", "c"}})
}

func TestFileReload(t *testing.T) {
	t.Parallel()

	const file = `cat: dog`

	f, err := ioutil.TempFile("", "maddy-tests-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(file); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	mod, err := NewFile("", "", nil, []string{f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*File)
	m.log = testutils.Logger(t, "file_map")
	defer m.Close()

	if err := mod.Init(&config.Map{Block: config.Node{}}); err != nil {
		t.Fatal(err)
	}

	// ensure it is correctly loaded at first time.
	m.mLck.RLock()
	if m.m["cat"] == nil {
		t.Fatalf("wrong content loaded, new m were not loaded, %v", m.m)
	}
	m.mLck.RUnlock()

	for i := 0; i < 100; i++ {
		// try to provoke race condition on file writing
		if i%2 == 0 {
			if err := os.WriteFile(f.Name(), []byte("dog: cat"), os.ModePerm); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(reloadInterval + 5*time.Millisecond)
		m.mLck.RLock()
		if m.m["dog"] == nil {
			t.Fatalf("wrong content loaded, new m were not loaded, %v", m.m)
		}
		m.mLck.RUnlock()
	}
}

func TestFileReload_Broken(t *testing.T) {
	t.Parallel()

	const file = `cat: dog`

	f, err := ioutil.TempFile("", "maddy-tests-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(file); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	mod, err := NewFile("", "", nil, []string{f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*File)
	m.log = testutils.Logger(t, FileModName)
	defer m.Close()

	if err := mod.Init(&config.Map{Block: config.Node{}}); err != nil {
		t.Fatal(err)
	}

	f2, err := os.OpenFile(f.Name(), os.O_WRONLY|os.O_SYNC, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f2.WriteString(":"); err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	time.Sleep(3 * reloadInterval)

	m.mLck.RLock()
	defer m.mLck.RUnlock()
	if m.m["cat"] == nil {
		t.Fatal("New m were loaded or map changed", m.m)
	}
}

func TestFileReload_Removed(t *testing.T) {
	t.Parallel()

	const file = `cat: dog`

	f, err := ioutil.TempFile("", "maddy-tests-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(file); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	mod, err := NewFile("", "", nil, []string{f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*File)
	m.log = testutils.Logger(t, FileModName)
	defer m.Close()

	if err := mod.Init(&config.Map{Block: config.Node{}}); err != nil {
		t.Fatal(err)
	}

	os.Remove(f.Name())

	time.Sleep(3 * reloadInterval)

	m.mLck.RLock()
	defer m.mLck.RUnlock()
	if m.m["cat"] != nil {
		t.Fatal("Old m are still loaded")
	}
}

func init() {
	reloadInterval = 10 * time.Millisecond
}
