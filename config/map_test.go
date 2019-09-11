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

	m := NewMap(nil, &cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(nil, &cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(map[string]interface{}{"foo": "bar"}, &cfg)

	foo := ""
	m.Custom("foo", true, true, nil, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(map[string]interface{}{}, &cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(map[string]interface{}{}, &cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(nil, &cfg)

	foo := ""
	m.Custom("foo", false, false, func() (interface{}, error) {
		return "bar", nil
	}, func(_ *Map, n *Node) (interface{}, error) {
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

	m := NewMap(map[string]interface{}{"foo": "baz"}, &cfg)

	foo := ""
	m.Custom("foo", true, false, func() (interface{}, error) {
		return "bar", nil
	}, func(_ *Map, n *Node) (interface{}, error) {
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
		_, err := m.ProcessWith(map[string]interface{}{}, &cfg)
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

	m := NewMap(nil, &cfg)

	foo := ""
	m.Custom("foo", false, true, nil, func(_ *Map, n *Node) (interface{}, error) {
		return n.Args[0], nil
	}, &foo)

	_, err := m.Process()
	if err == nil {
		t.Errorf("Expected failure")
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

	m := NewMap(nil, &cfg)

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

	m := NewMap(nil, &cfg)

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

	m := NewMap(nil, &cfg)

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

	m := NewMap(nil, &cfg)

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

	m := NewMap(nil, &cfg)

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
