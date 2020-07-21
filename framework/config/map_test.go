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

package config

import (
	"testing"
)

func TestMapProcess(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"bar"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != "bar" {
		t.Errorf("Incorrect value stored in variable, want 'bar', got '%s'", foo)
	}
}

func TestMapProcess_MissingRequired(t *testing.T) {
	cfg := Node{
		Children: []Node{},
	}

	m := NewMap(nil, cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}
}

func TestMapProcess_InheritGlobal(t *testing.T) {
	cfg := Node{
		Children: []Node{},
	}

	m := NewMap(map[string]interface{}{"foo": "bar"}, cfg)

	foo := ""
	m.Custom("foo", true, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != "bar" {
		t.Errorf("Incorrect value stored in variable, want 'bar', got '%s'", foo)
	}
}

func TestMapProcess_InheritGlobal_MissingRequired(t *testing.T) {
	cfg := Node{
		Children: []Node{},
	}

	m := NewMap(map[string]interface{}{}, cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}
}

func TestMapProcess_InheritGlobal_Override(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"bar"},
			},
		},
	}

	m := NewMap(map[string]interface{}{}, cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != "bar" {
		t.Errorf("Incorrect value stored in variable, want 'bar', got '%s'", foo)
	}
}

func TestMapProcess_DefaultValue(t *testing.T) {
	cfg := Node{
		Children: []Node{},
	}

	m := NewMap(nil, cfg)

	foo := ""
	m.Custom("foo", false, false, func() (interface{}, error) {
		return "bar", nil
	}, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != "bar" {
		t.Errorf("Incorrect value stored in variable, want 'bar', got '%s'", foo)
	}
}

func TestMapProcess_InheritGlobal_DefaultValue(t *testing.T) {
	cfg := Node{
		Children: []Node{},
	}

	m := NewMap(map[string]interface{}{"foo": "baz"}, cfg)

	foo := ""
	m.Custom("foo", true, false, func() (interface{}, error) {
		return "bar", nil
	}, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != "baz" {
		t.Errorf("Incorrect value stored in variable, want 'baz', got '%s'", foo)
	}

	t.Run("no global", func(t *testing.T) {
		_, err := m.ProcessWith(map[string]interface{}{}, cfg)
		if err != nil {
			t.Fatalf("Unexpected failure: %v", err)
		}

		if foo != "bar" {
			t.Errorf("Incorrect value stored in variable, want 'bar', got '%s'", foo)
		}
	})
}

func TestMapProcess_Duplicate(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"bar"},
			},
			{
				Name: "foo",
				Args: []string{"bar"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}
}

func TestMapProcess_Unexpected(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"baz"},
			},
			{
				Name: "bar",
				Args: []string{"baz"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := ""
	m.Custom("bar", false, true, nil, func(_ *Map, n Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}

	m.AllowUnknown()

	unknown, err := m.Process()
	if err != nil {
		t.Errorf("Unexpected failure: %v", err)
	}

	if len(unknown) != 1 {
		t.Fatalf("Wrong amount of unknown nodes: %v", len(unknown))
	}

	if unknown[0].Name != "foo" {
		t.Fatalf("Wrong node in unknown: %v", unknown[0].Name)
	}
}

func TestMapInt(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"1"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := 0
	m.Int("foo", false, true, 0, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != 1 {
		t.Errorf("Incorrect value stored in variable, want 1, got %d", foo)
	}
}

func TestMapInt_Invalid(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"AAAA"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := 0
	m.Int("foo", false, true, 0, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}
}

func TestMapFloat(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"1"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := 0.0
	m.Float("foo", false, true, 0, &foo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if foo != 1.0 {
		t.Errorf("Incorrect value stored in variable, want 1, got %v", foo)
	}
}

func TestMapFloat_Invalid(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
				Args: []string{"AAAA"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo := 0.0
	m.Float("foo", false, true, 0, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
	}
}

func TestMapBool(t *testing.T) {
	cfg := Node{
		Children: []Node{
			{
				Name: "foo",
			},
			{
				Name: "bar",
				Args: []string{"yes"},
			},
			{
				Name: "baz",
				Args: []string{"no"},
			},
		},
	}

	m := NewMap(nil, cfg)

	foo, bar, baz, boo := false, false, false, false
	m.Bool("foo", false, false, &foo)
	m.Bool("bar", false, false, &bar)
	m.Bool("baz", false, false, &baz)
	m.Bool("boo", false, false, &boo)

	_, err := m.Process()
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}

	if !foo {
		t.Errorf("Incorrect value stored in variable foo, want true, got false")
	}
	if !bar {
		t.Errorf("Incorrect value stored in variable bar, want true, got false")
	}
	if baz {
		t.Errorf("Incorrect value stored in variable baz, want false, got true")
	}
	if boo {
		t.Errorf("Incorrect value stored in variable boo, want false, got true")
	}
}

func TestParseDataSize(t *testing.T) {
	check := func(s string, ok bool, expected int) {
		val, err := ParseDataSize(s)
		if err != nil && ok {
			t.Errorf("unexpected parseDataSize('%s') fail: %v", s, err)
			return
		}
		if err == nil && !ok {
			t.Errorf("unexpected parseDataSize('%s') success, got %d", s, val)
			return
		}
		if val != expected {
			t.Errorf("parseDataSize('%s') != %d", s, expected)
			return
		}
	}

	check("1M", true, 1024*1024)
	check("1K", true, 1024)
	check("1b", true, 1)
	check("1M 5b", true, 1024*1024+5)
	check("1M 5K 5b", true, 1024*1024+5*1024+5)
	check("0", true, 0)
	check("1", false, 0)
	check("1d", false, 0)
	check("d", false, 0)
	check("unrelated", false, 0)
	check("1M5b", false, 0)
	check("", false, 0)
	check("-5M", false, 0)
}

func TestMap_Callback(t *testing.T) {
	called := map[string]int{}

	cfg := Node{
		Children: []Node{
			{
				Name: "test2",
				Args: []string{"a"},
			},
			{
				Name: "test3",
				Args: []string{"b"},
			},
			{
				Name: "test3",
				Args: []string{"b"},
			},
			{
				Name: "unrelated",
				Args: []string{"b"},
			},
		},
	}
	m := NewMap(nil, cfg)
	m.Callback("test1", func(*Map, Node) error {
		called["test1"]++
		return nil
	})
	m.Callback("test2", func(_ *Map, n Node) error {
		called["test2"]++
		if n.Args[0] != "a" {
			t.Fatal("Wrong n.Args[0] for test2:", n.Args[0])
		}
		return nil
	})
	m.Callback("test3", func(_ *Map, n Node) error {
		called["test3"]++
		if n.Args[0] != "b" {
			t.Fatal("Wrong n.Args[0] for test2:", n.Args[0])
		}
		return nil
	})
	m.AllowUnknown()
	others, err := m.Process()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if called["test1"] != 0 {
		t.Error("test1 CB was called when it should not")
	}
	if called["test2"] != 1 {
		t.Error("test2 CB was not called when it should")
	}
	if called["test3"] != 2 {
		t.Error("test3 CB was not called when it should")
	}
	if len(others) != 1 {
		t.Error("Wrong amount of unmatched directives")
	}
	if others[0].Name != "unrelated" {
		t.Error("Wrong directive returned in unmatched slice:", others[0].Name)
	}
}
