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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type matcher struct {
	name          string
	required      bool
	inheritGlobal bool
	defaultVal    func() (interface{}, error)
	mapper        func(*Map, Node) (interface{}, error)
	store         *reflect.Value

	customCallback func(*Map, Node) error
}

func (m *matcher) assign(val interface{}) {
	valRefl := reflect.ValueOf(val)
	// Convert untyped nil into typed nil. Otherwise it will panic.
	if !valRefl.IsValid() {
		valRefl = reflect.Zero(m.store.Type())
	}

	m.store.Set(valRefl)
}

// Map structure implements reflection-based conversion between configuration
// directives and Go variables.
type Map struct {
	allowUnknown bool

	// All values saved by Map during processing.
	Values map[string]interface{}

	entries map[string]matcher

	// Values used by Process as default values if inheritGlobal is true.
	Globals map[string]interface{}
	// Config block used by Process.
	Block Node
}

func NewMap(globals map[string]interface{}, block Node) *Map {
	return &Map{Globals: globals, Block: block}
}

// AllowUnknown makes config.Map skip unknown configuration directives instead
// of failing.
func (m *Map) AllowUnknown() {
	m.allowUnknown = true
}

// EnumList maps a configuration directive to a []string variable.
//
// Directive must be in form 'name string1 string2' where each string should be from *allowed*
// slice. At least one argument should be present.
//
// See Map.Custom for description of inheritGlobal and required.
func (m *Map) EnumList(name string, inheritGlobal, required bool, allowed, defaultVal []string, store *[]string) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare a block here")
		}
		if len(node.Args) == 0 {
			return nil, NodeErr(node, "expected at least one argument")
		}

		for _, arg := range node.Args {
			isAllowed := false
			for _, str := range allowed {
				if str == arg {
					isAllowed = true
				}
			}
			if !isAllowed {
				return nil, NodeErr(node, "invalid argument, valid values are: %v", allowed)
			}
		}

		return node.Args, nil
	}, store)
}

// Enum maps a configuration directive to a string variable.
//
// Directive must be in form 'name string' where string should be from *allowed*
// slice. That string argument will be stored in store variable.
//
// See Map.Custom for description of inheritGlobal and required.
func (m *Map) Enum(name string, inheritGlobal, required bool, allowed []string, defaultVal string, store *string) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare a block here")
		}
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected exactly one argument")
		}

		for _, str := range allowed {
			if str == node.Args[0] {
				return node.Args[0], nil
			}
		}

		return nil, NodeErr(node, "invalid argument, valid values are: %v", allowed)
	}, store)
}

// EnumMapped is similar to Map.Enum but maps a stirng to a custom type.
func EnumMapped[V any](m *Map, name string, inheritGlobal, required bool, mapped map[string]V, defaultVal V, store *V) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare a block here")
		}
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected exactly one argument")
		}

		val, ok := mapped[node.Args[0]]
		if !ok {
			validValues := make([]string, 0, len(mapped))
			for k := range mapped {
				validValues = append(validValues, k)
			}
			return nil, NodeErr(node, "invalid argument, valid values are: %v", validValues)
		}

		return val, nil
	}, store)
}

// EnumListMapped is similar to Map.EnumList but maps a stirng to a custom type.
func EnumListMapped[V any](m *Map, name string, inheritGlobal, required bool, mapped map[string]V, defaultVal []V, store *[]V) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare a block here")
		}
		if len(node.Args) == 0 {
			return nil, NodeErr(node, "expected at least one argument")
		}

		values := make([]V, 0, len(node.Args))
		for _, arg := range node.Args {
			val, ok := mapped[arg]
			if !ok {
				validValues := make([]string, 0, len(mapped))
				for k := range mapped {
					validValues = append(validValues, k)
				}
				return nil, NodeErr(node, "invalid argument, valid values are: %v", validValues)
			}
			values = append(values, val)
		}
		return values, nil
	}, store)
}

// Duration maps configuration directive to a time.Duration variable.
//
// Directive must be in form 'name duration' where duration is any string accepted by
// time.ParseDuration. As an additional requirement, result of time.ParseDuration must not
// be negative.
//
// Note that for convenience, if directive does have multiple arguments, they will be joined
// without separators. E.g. 'name 1h 2m' will become 'name 1h2m' and so '1h2m' will be passed
// to time.ParseDuration.
//
// See Map.Custom for description of arguments.
func (m *Map) Duration(name string, inheritGlobal, required bool, defaultVal time.Duration, store *time.Duration) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}
		if len(node.Args) == 0 {
			return nil, NodeErr(node, "at least one argument is required")
		}

		durationStr := strings.Join(node.Args, "")
		dur, err := time.ParseDuration(durationStr)
		if err != nil {
			return nil, NodeErr(node, "%v", err)
		}

		if dur < 0 {
			return nil, NodeErr(node, "duration must not be negative")
		}

		return dur, nil
	}, store)
}

func ParseDataSize(s string) (int, error) {
	if len(s) == 0 {
		return 0, errors.New("missing a number")
	}

	// ' ' terminates the number+suffix pair.
	s = s + " "

	var total int
	currentDigit := ""
	suffix := ""
	for _, ch := range s {
		if unicode.IsDigit(ch) {
			if suffix != "" {
				return 0, errors.New("unexpected digit after a suffix")
			}
			currentDigit += string(ch)
			continue
		}
		if ch != ' ' {
			suffix += string(ch)
			continue
		}

		num, err := strconv.Atoi(currentDigit)
		if err != nil {
			return 0, err
		}

		if num < 0 {
			return 0, errors.New("value must not be negative")
		}

		switch suffix {
		case "G":
			total += num * 1024 * 1024 * 1024
		case "M":
			total += num * 1024 * 1024
		case "K":
			total += num * 1024
		case "B", "b":
			total += num
		default:
			if num != 0 {
				return 0, errors.New("unknown unit suffix: " + suffix)
			}
		}

		suffix = ""
		currentDigit = ""
	}

	return total, nil
}

// DataSize maps configuration directive to a int variable, representing data size.
//
// Syntax requires unit suffix to be added to the end of string to specify
// data unit and allows multiple arguments (they will be added together).
//
// See Map.Custom for description of arguments.
func (m *Map) DataSize(name string, inheritGlobal, required bool, defaultVal int64, store *int64) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}
		if len(node.Args) == 0 {
			return nil, NodeErr(node, "at least one argument is required")
		}

		durationStr := strings.Join(node.Args, " ")
		dur, err := ParseDataSize(durationStr)
		if err != nil {
			return nil, NodeErr(node, "%v", err)
		}

		return int64(dur), nil
	}, store)
}

func ParseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "1", "true", "on", "yes":
		return true, nil
	case "0", "false", "off", "no":
		return false, nil
	}
	return false, fmt.Errorf("bool argument should be 'yes' or 'no'")
}

// Bool maps presence of some configuration directive to a boolean variable.
// Additionally, 'name yes' and 'name no' are mapped to true and false
// correspondingly.
//
// I.e. if directive 'io_debug' exists in processed configuration block or in
// the global configuration (if inheritGlobal is true) then Process will store
// true in target variable.
func (m *Map) Bool(name string, inheritGlobal, defaultVal bool, store *bool) {
	m.Custom(name, inheritGlobal, false, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		if len(node.Args) == 0 {
			return true, nil
		}
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected exactly 1 argument")
		}

		b, err := ParseBool(node.Args[0])
		if err != nil {
			return nil, NodeErr(node, "bool argument should be 'yes' or 'no'")
		}
		return b, nil
	}, store)
}

// StringList maps configuration directive with the specified name to variable
// referenced by 'store' pointer.
//
// Configuration directive must be in form 'name arbitrary_string arbitrary_string ...'
// Where at least one argument must be present.
//
// See Custom function for details about inheritGlobal, required and
// defaultVal.
func (m *Map) StringList(name string, inheritGlobal, required bool, defaultVal []string, store *[]string) {
	m.Custom(name, inheritGlobal, required, func() (interface{}, error) {
		return defaultVal, nil
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) == 0 {
			return nil, NodeErr(node, "expected at least one argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		return node.Args, nil
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.Atoi(node.Args[0])
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 32)
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.ParseInt(node.Args[0], 10, 32)
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 32)
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.ParseInt(node.Args[0], 10, 64)
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}
		if len(node.Children) != 0 {
			return nil, NodeErr(node, "can't declare block here")
		}

		i, err := strconv.ParseUint(node.Args[0], 10, 64)
		if err != nil {
			return nil, NodeErr(node, "invalid integer: %s", node.Args[0])
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
	}, func(_ *Map, node Node) (interface{}, error) {
		if len(node.Args) != 1 {
			return nil, NodeErr(node, "expected 1 argument")
		}

		f, err := strconv.ParseFloat(node.Args[0], 64)
		if err != nil {
			return nil, NodeErr(node, "invalid float: %s", node.Args[0])
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
// Note that if inheritGlobal is true, defaultVal of the global directive
// will be used instead.
//
// mapper is a function that should convert configuration directive arguments
// into variable value.  Both functions may fail with errors, configuration
// processing will stop immediately then.
// Note: mapper function should not modify passed values.
//
// store is where the value returned by mapper should be stored. Can be nil
// (value will be saved only in Map.Values).
func (m *Map) Custom(name string, inheritGlobal, required bool, defaultVal func() (interface{}, error), mapper func(*Map, Node) (interface{}, error), store interface{}) {
	if m.entries == nil {
		m.entries = make(map[string]matcher)
	}
	if _, ok := m.entries[name]; ok {
		panic("Map.Custom: duplicate matcher")
	}

	var target *reflect.Value
	ptr := reflect.ValueOf(store)
	if ptr.IsValid() && !ptr.IsNil() {
		val := ptr.Elem()
		if !val.CanSet() {
			panic("Map.Custom: store argument must be settable (a pointer)")
		}
		target = &val
	}

	m.entries[name] = matcher{
		name:          name,
		inheritGlobal: inheritGlobal,
		required:      required,
		defaultVal:    defaultVal,
		mapper:        mapper,
		store:         target,
	}
}

// Callback creates mapping that will call mapper() function for each
// directive with the specified name. No further processing is done.
//
// Directives with the specified name will not be returned by Process if
// AllowUnknown is used.
//
// It is intended to permit multiple independent values of directive with
// implementation-defined handling.
func (m *Map) Callback(name string, mapper func(*Map, Node) error) {
	if m.entries == nil {
		m.entries = make(map[string]matcher)
	}
	if _, ok := m.entries[name]; ok {
		panic("Map.Custom: duplicate matcher")
	}

	m.entries[name] = matcher{
		name:           name,
		customCallback: mapper,
	}
}

// Process maps variables from global configuration and block passed in NewMap.
//
// If Map instance was not created using NewMap - Process panics.
func (m *Map) Process() (unknown []Node, err error) {
	return m.ProcessWith(m.Globals, m.Block)
}

// Process maps variables from global configuration and block passed in arguments.
func (m *Map) ProcessWith(globalCfg map[string]interface{}, block Node) (unknown []Node, err error) {
	unknown = make([]Node, 0, len(block.Children))
	matched := make(map[string]bool)
	m.Values = make(map[string]interface{})

	for _, subnode := range block.Children {
		matcher, ok := m.entries[subnode.Name]
		if !ok {
			if !m.allowUnknown {
				return nil, NodeErr(subnode, "unexpected directive: %s", subnode.Name)
			}
			unknown = append(unknown, subnode)
			continue
		}

		if matcher.customCallback != nil {
			if err := matcher.customCallback(m, subnode); err != nil {
				return nil, err
			}
			matched[subnode.Name] = true
			continue
		}

		if matched[subnode.Name] {
			return nil, NodeErr(subnode, "duplicate directive: %s", subnode.Name)
		}
		matched[subnode.Name] = true

		val, err := matcher.mapper(m, subnode)
		if err != nil {
			return nil, err
		}
		m.Values[matcher.name] = val
		if matcher.store != nil {
			matcher.assign(val)
		}
	}

	for _, matcher := range m.entries {
		if matched[matcher.name] {
			continue
		}
		if matcher.mapper == nil {
			continue
		}

		var val interface{}
		globalVal, ok := globalCfg[matcher.name]
		if matcher.inheritGlobal && ok {
			val = globalVal
		} else if !matcher.required {
			if matcher.defaultVal == nil {
				continue
			}

			val, err = matcher.defaultVal()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, NodeErr(block, "missing required directive: %s", matcher.name)
		}

		// If we put zero values into map then code that checks globalCfg
		// above will inherit them for required fields instead of failing.
		//
		// This is important for fields that are required to be specified
		// either globally or on per-block basis (e.g. tls, hostname).
		// For these directives, global Map does have required = false
		// so global values are default which is usually zero value.
		//
		// This is a temporary solutions, of course, in the long-term
		// the way global values and "inheritance" is handled should be
		// revised.
		store := false
		valT := reflect.TypeOf(val)
		if valT != nil {
			zero := reflect.Zero(valT)
			store = !reflect.DeepEqual(val, zero.Interface())
		}

		if store {
			m.Values[matcher.name] = val
		}
		if matcher.store != nil {
			matcher.assign(val)
		}
	}

	return unknown, nil
}
