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

package parser

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

var cases = []struct {
	name string
	cfg  string
	tree []Node
	fail bool
}{
	{
		"single directive without args",
		`a`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"single directive with args",
		`a a1 a2`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"a1", "a2"},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"single directive with empty braces",
		`a { }`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{},
				Children: []Node{},
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"single directive with arguments and empty braces",
		`a a1 a2 { }`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"a1", "a2"},
				Children: []Node{},
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"single directive with a block",
		`a a1 a2 {
			a_child1 c1arg1 c1arg2
			a_child2 c2arg1 c2arg2
		}`,
		[]Node{
			{
				Name: "a",
				Args: []string{"a1", "a2"},
				Children: []Node{
					{
						Name:     "a_child1",
						Args:     []string{"c1arg1", "c1arg2"},
						Children: nil,
						File:     "test",
						Line:     2,
					},
					{
						Name:     "a_child2",
						Args:     []string{"c2arg1", "c2arg2"},
						Children: nil,
						File:     "test",
						Line:     3,
					},
				},
				File: "test",
				Line: 1,
			},
		},
		false,
	},
	{
		"single directive with missing closing brace",
		`a {`,
		nil,
		true,
	},
	{
		"single directive with missing opening brace",
		`a }`,
		nil,
		true,
	},
	{
		"two directives",
		`a
		 b`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     1,
			},
			{
				Name:     "b",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     2,
			},
		},
		false,
	},
	{
		"two directives with arguments",
		`a a1 a2
		 b b1 b2`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"a1", "a2"},
				Children: nil,
				File:     "test",
				Line:     1,
			},
			{
				Name:     "b",
				Args:     []string{"b1", "b2"},
				Children: nil,
				File:     "test",
				Line:     2,
			},
		},
		false,
	},
	{
		"backslash on the end of line",
		`a a1 a2 \
		   a3 a4`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"a1", "a2", "a3", "a4"},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"directive with missing closing brace on different line",
		`a a1 a2 {
			a_child1 c1arg1 c1arg2
		`,
		nil,
		true,
	},
	{
		"single directive with closing brace on children's line",
		`a a1 a2 {
			a_child1 c1arg1 c1arg2
			a_child2 c2arg1 c2arg2 }
		 b`,
		[]Node{
			{
				Name: "a",
				Args: []string{"a1", "a2"},
				Children: []Node{
					{
						Name:     "a_child1",
						Args:     []string{"c1arg1", "c1arg2"},
						Children: nil,
						File:     "test",
						Line:     2,
					},
					{
						Name:     "a_child2",
						Args:     []string{"c2arg1", "c2arg2"},
						Children: nil,
						File:     "test",
						Line:     3,
					},
				},
				File: "test",
				Line: 1,
			},
			{
				Name:     "b",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     4,
			},
		},
		false,
	},
	{
		"single directive with childrens on the same line",
		`a a1 a2 { a_child1 c1arg1 c1arg2 }`,
		[]Node{
			{
				Name: "a",
				Args: []string{"a1", "a2"},
				Children: []Node{
					{
						Name:     "a_child1",
						Args:     []string{"c1arg1", "c1arg2"},
						Children: nil,
						File:     "test",
						Line:     1,
					},
				},
				File: "test",
				Line: 1,
			},
		},
		false,
	},
	{
		"invalid directive name",
		`a-a4@%8 whatever`,
		nil,
		true,
	},
	{
		"directive name starts with a digit",
		`1w whatever`,
		nil,
		true,
	},
	{
		"missing block header",
		`{ a_child1 c1arg1 c1arg2 }`,
		nil,
		true,
	},
	{
		"extra closing brace",
		`a {
			child1
		} }
		`,
		nil,
		true,
	},
	{
		"extra opening brace",
		`a { {
		}`,
		nil,
		true,
	},
	{
		"closing brace in next block header",
		`a {
		} b b1`,
		nil,
		true,
	},
	{
		"environment variable expansion",
		`a {env:TESTING_VARIABLE}`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"ABCDEF"},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"missing environment variable expansion (unix-like syntax)",
		`a {env:TESTING_VARIABLE3}`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{""},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"incomplete environment variable syntax",
		`a {env:TESTING_VARIABLE`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{"{env:TESTING_VARIABLE"},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"snippet expansion",
		`(foo) { a }
		 import foo`,
		[]Node{
			{
				Name:     "a",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"snippet expansion inside a block",
		`(foo) { a }
        foo {
            boo
            import foo
        }`,
		[]Node{
			{
				Name: "foo",
				Args: []string{},
				Children: []Node{
					{
						Name: "boo",
						Args: []string{},
						File: "test",
						Line: 3,
					},
					{
						Name: "a",
						Args: []string{},
						File: "test",
						Line: 1,
					},
				},
				File: "test",
				Line: 2,
			},
		},
		false,
	},
	{
		"missing snippet",
		`import foo`,
		nil,
		true,
	},
	{
		"unlimited recursive snippet expansion",
		`(foo) { import foo }
		 import foo`,
		nil,
		true,
	},
	{
		"snippet declaration with args",
		`(foo) a b c { }`,
		nil,
		true,
	},
	{
		"snippet declaration inside block",
		`abc {
			(foo) { }
		}`,
		nil,
		true,
	},
	{
		"block nesting limit",
		`a ` + strings.Repeat("a { ", 1000) + strings.Repeat(" }", 1000),
		nil,
		true,
	},
	{
		"macro expansion, single argument",
		`$(foo) = bar
		dir $(foo)`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{"bar"},
				Children: nil,
				File:     "test",
				Line:     2,
			},
		},
		false,
	},
	{
		"macro expansion, inside argument",
		`$(foo) = bar
		dir aaa/$(foo)/bbb`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{"aaa/bar/bbb"},
				Children: nil,
				File:     "test",
				Line:     2,
			},
		},
		false,
	},
	{
		"macro expansion, inside argument, multi-value",
		`$(foo) = bar baz
		dir aaa/$(foo)/bbb`,
		nil,
		true,
	},
	{
		"macro expansion, multiple arguments",
		`$(foo) = bar baz
		dir $(foo)`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{"bar", "baz"},
				Children: nil,
				File:     "test",
				Line:     2,
			},
		},
		false,
	},
	{
		"macro expansion, undefined",
		`dir $(foo)`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     1,
			},
		},
		false,
	},
	{
		"macro expansion, empty",
		`$(foo) =`,
		nil,
		true,
	},
	{
		"macro expansion, name replacement",
		`$(foo) = a b
			$(foo) 1`,
		nil,
		true,
	},
	{
		"macro expansion, missing =",
		`$(foo) a b
			$(foo) 1`,
		nil,
		true,
	},
	{
		"macro expansion, not on top level",
		`a {
				$(foo) = a b
			}
			$(foo) 1`,
		nil,
		true,
	},
	{
		"macro expansion, nested",
		`$(foo) = a
			$(bar) = $(foo) b
			dir $(bar)`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{"a", "b"},
				Children: nil,
				File:     "test",
				Line:     3,
			},
		},
		false,
	},
	{
		"macro expansion, used inside snippet",
		`$(foo) = a
			(bar) {
				dir $(foo)
			}
			import bar`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{"a"},
				Children: nil,
				File:     "test",
				Line:     3,
			},
		},
		false,
	},
	{
		"macro expansion, used inside snippet, defined after",
		`
			(bar) {
				dir $(foo)
			}
			$(foo) = a
			import bar`,
		[]Node{
			{
				Name:     "dir",
				Args:     []string{},
				Children: nil,
				File:     "test",
				Line:     3,
			},
		},
		false,
	},
}

func printTree(t *testing.T, root Node, indent int) {
	t.Log(strings.Repeat(" ", indent)+root.Name, root.Args)
	for _, child := range root.Children {
		t.Log(child, indent+1)
	}
}

func TestRead(t *testing.T) {
	os.Setenv("TESTING_VARIABLE", "ABCDEF")
	os.Setenv("TESTING_VARIABLE2", "ABC2 DEF2")

	for _, case_ := range cases {
		t.Run(case_.name, func(t *testing.T) {
			tree, err := Read(strings.NewReader(case_.cfg), "test")
			if !case_.fail && err != nil {
				t.Error("unexpected failure:", err)
				return
			}
			if case_.fail {
				if err == nil {
					t.Log("expected failure but Read succeeded")
					t.Log("got tree:")
					t.Logf("%+v", tree)
					for _, node := range tree {
						printTree(t, node, 0)
					}
					t.Fail()
					return
				}
				return
			}

			if !reflect.DeepEqual(case_.tree, tree) {
				t.Log("parse result mismatch")
				t.Log("expected:")
				t.Logf("%+#v", case_.tree)
				for _, node := range case_.tree {
					printTree(t, node, 0)
				}
				t.Log("actual:")
				t.Logf("%+#v", tree)
				for _, node := range tree {
					printTree(t, node, 0)
				}
				t.Fail()
			}
		})
	}
}
