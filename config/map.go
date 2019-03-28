package config

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

type matcher struct {
	name          string
	required      bool
	inheritGlobal bool
	defaultVal    func() (interface{}, error)
	mapper        func(*Map, *Node) (interface{}, error)
	store         reflect.Value
}

func (m *matcher) match(map_ *Map, node *Node) error {
	val, err := m.mapper(map_, node)
	if err != nil {
		return err
	}

	valRefl := reflect.ValueOf(val)
	// Convert untyped nil into typed nil. Otherwise it will panic.
	if !valRefl.IsValid() {
		valRefl = reflect.Zero(m.store.Type())
	}

	m.store.Set(valRefl)
	return nil
}

// Map structure implements reflection-based conversion between configuration
// directives and Go variables.
type Map struct {
	allowUnknown bool

	// Set to currently processed node when defaultVal or mapper functions are
	// called.
	curNode *Node

	entries map[string]matcher
}

// MatchErr returns error with formatted message, if called from defaultVal or
// mapper functions - message will be prepended with information about
// processed config node.
func (m *Map) MatchErr(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)

	if m.curNode != nil {
		return fmt.Errorf("%s:%d %s: %s", m.curNode.File, m.curNode.Line, m.curNode.Name, msg)
	} else {
		return errors.New(msg)
	}
}

// AllowUnknown makes config.Map skip unknown configuration directives instead
// of failing.
func (m *Map) AllowUnknown() {
	m.allowUnknown = true
}

// Bool maps presence of some configuration directive to a boolean variable.
//
// I.e. if directive 'io_debug' exists in processed configuration block or in
// the global configuration (if inheritGlobal is true) then Process will store
// true in target variable.
func (m *Map) Bool(name string, inheritGlobal bool, store *bool) {
	m.Custom(name, inheritGlobal, false, func() (interface{}, error) {
		return false, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 0 {
			return nil, m.MatchErr("unexpected arguments")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		return true, nil
	}, store)
}

// String maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name arbitrary_string'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) String(name string, inheritGlobal, required bool, defaultVal string, store *string) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		return node.Args[0], nil
	}, store)
}

// Int maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) Int(name string, inheritGlobal, required bool, defaultVal int, store *int) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.Atoi(node.Args[0])
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return i, nil
	}, store)
}

// UInt maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) UInt(name string, inheritGlobal, required bool, defaultVal uint, store *uint) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 32)
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return uint(i), nil
	}, store)
}

// Int32 maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) Int32(name string, inheritGlobal, required bool, defaultVal int32, store *int32) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.ParseInt(node.Args[0], 10, 32)
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return int32(i), nil
	}, store)
}

// UInt32 maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) UInt32(name string, inheritGlobal, required bool, defaultVal uint32, store *uint32) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 32)
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return uint32(i), nil
	}, store)
}

// Int64 maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) Int64(name string, inheritGlobal, required bool, defaultVal int64, store *int64) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.ParseInt(node.Args[0], 10, 64)
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return i, nil
	}, store)
}

// UInt64 maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) UInt64(name string, inheritGlobal, required bool, defaultVal uint64, store *uint64) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, m.MatchErr("can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 64)
		if err != nil {
			return nil, m.MatchErr("invalid integer: %s", node.Args[0])
		}
		return i, nil
	}, store)
}

// Float maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name 123.55'.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) Float(name string, inheritGlobal, required bool, defaultVal float64, store *float64) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(m *Map, node *Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, m.MatchErr("expected 1 argument")
		}

		f, err := strconv.ParseFloat(node.Args[0], 64)
		if err != nil {
			return nil, m.MatchErr("invalid float: %s", node.Args[0])
		}
		return f, nil
	}, store)
}

// Custom maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// If inheritGlobal is true - Map will try to use a value from globalCfg if
// none is set in a processed configuration block.
//
// If required is true - Map will fail if no value is set in the configuration,
// both global (if inheritGlobal is true) and in the processed block.
//
// defaultVal is a factory function that should return the default value for
// the variable. It will be used if no value is set in the config. It can be
// nil if required is true.
//
// mapper is a function that should convert configuration directive arguments
// into variable value.  Both functions may fail with errors, configuration
// processing will stop immediately then.
func (m *Map) Custom(name string, inheritGlobal, required bool, defaultVal func() (interface{}, error), mapper func(*Map, *Node) (interface{}, error), store interface{}) {
	if m.entries == nil {
		m.entries = make(map[string]matcher)
	}

	val := reflect.ValueOf(store).Elem()
	if !val.CanSet() {
		panic("Map.Custom: store argument must be settable (a pointer)")
	}

	if _, ok := m.entries[name]; ok {
		panic("Map.Custom: duplicate matcher")
	}

	m.entries[name] = matcher{
		name:          name,
		inheritGlobal: inheritGlobal,
		required:      required,
		defaultVal:    defaultVal,
		mapper:        mapper,
		store:         val,
	}
}

func (m *Map) Process(globalCfg map[string]Node, tree *Node) (unmatched []Node, err error) {
	unmatched = make([]Node, 0, len(tree.Children))
	matched := make(map[string]bool)

	for _, subnode := range tree.Children {
		m.curNode = &subnode

		if matched[subnode.Name] {
			return nil, m.MatchErr("duplicate directive")
		}

		matcher, ok := m.entries[subnode.Name]
		if !ok {
			if !m.allowUnknown {
				return nil, m.MatchErr("unexpected directive")
			}
			unmatched = append(unmatched, subnode)
			continue
		}

		if err := matcher.match(m, m.curNode); err != nil {
			return nil, err
		}
		matched[subnode.Name] = true
	}
	m.curNode = nil

	for _, matcher := range m.entries {
		if matched[matcher.name] {
			continue
		}

		globalNode, ok := globalCfg[matcher.name]
		if matcher.inheritGlobal && ok {
			m.curNode = &globalNode
			if err := matcher.match(m, m.curNode); err != nil {
				m.curNode = nil
				return nil, err
			}
			m.curNode = nil
		} else if !matcher.required {
			if matcher.defaultVal == nil {
				continue
			}

			val, err := matcher.defaultVal()
			if err != nil {
				return nil, err
			}
			if val == nil {
				return nil, m.MatchErr("missing required directive: %s", matcher.name)
			}

			valRefl := reflect.ValueOf(val)
			// Convert untyped nil into typed nil. Otherwise it will panic.
			if !valRefl.IsValid() {
				valRefl = reflect.Zero(matcher.store.Type())
			}

			matcher.store.Set(valRefl)
			continue
		} else {
			return nil, m.MatchErr("missing required directive: %s", matcher.name)
		}
	}

	return unmatched, nil
}
