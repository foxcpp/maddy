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

// Package config provides set of utilities for configuration parsing.
package parser

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/foxcpp/maddy/framework/config/lexer"
)

// Node struct describes a parsed configurtion block or a simple directive.
//
// name arg0 arg1 {
//  children0
//  children1
// }
type Node struct {
	// Name is the first string at node's line.
	Name string
	// Args are any strings placed after the node name.
	Args []string

	// Children slice contains all children blocks if node is a block. Can be nil.
	Children []Node

	// Snippet indicates whether current parsed node is a snippet. Always false
	// for all nodes returned from Read because snippets are expanded before it
	// returns.
	Snippet bool

	// Macro indicates whether current parsed node is a macro. Always false
	// for all nodes returned from Read because macros are expanded before it
	// returns.
	Macro bool

	// File is the name of node's source file.
	File string

	// Line is the line number where the directive is located in the source file. For
	// blocks this is the line where "block header" (name + args) resides.
	Line int
}

type parseContext struct {
	lexer.Dispenser
	nesting  int
	snippets map[string][]Node
	macros   map[string][]string

	fileLocation string
}

func validateNodeName(s string) error {
	if len(s) == 0 {
		return errors.New("empty directive name")
	}

	if unicode.IsDigit([]rune(s)[0]) {
		return errors.New("directive name cannot start with a digit")
	}

	allowedPunct := map[rune]bool{'.': true, '-': true, '_': true}

	for _, ch := range s {
		if !unicode.IsLetter(ch) &&
			!unicode.IsDigit(ch) &&
			!allowedPunct[ch] {
			return errors.New("character not allowed in directive name: " + string(ch))
		}
	}

	return nil
}

// readNode reads node starting at current token pointed by the lexer's
// cursor (it should point to node name).
//
// After readNode returns, the lexer's cursor will point to the last token of the parsed
// Node. This ensures predictable cursor location independently of the EOF state.
// Thus code reading multiple nodes should call readNode then manually
// advance lexer cursor (ctx.Next) and either call readNode again or stop
// because cursor hit EOF.
//
// readNode calls readNodes if currently parsed node is a block.
func (ctx *parseContext) readNode() (Node, error) {
	node := Node{}
	node.File = ctx.File()
	node.Line = ctx.Line()

	if ctx.Val() == "{" {
		return node, ctx.SyntaxErr("block header")
	}

	node.Name = ctx.Val()
	if ok, name := ctx.isSnippet(node.Name); ok {
		node.Name = name
		node.Snippet = true
	}

	var continueOnLF bool
	for {
		for ctx.NextArg() || (continueOnLF && ctx.NextLine()) {
			continueOnLF = false
			// name arg0 arg1 {
			//              # ^ called when we hit this token
			//   c0
			//   c1
			// }
			if ctx.Val() == "{" {
				var err error
				node.Children, err = ctx.readNodes()
				if err != nil {
					return node, err
				}
				break
			}

			node.Args = append(node.Args, ctx.Val())
		}

		// Continue reading the same Node if the \ was used to escape the newline.
		// E.g.
		//   name arg0 arg1 \
		//	   arg2 arg3
		if len(node.Args) != 0 && node.Args[len(node.Args)-1] == `\` {
			last := len(node.Args) - 1
			node.Args[last] = node.Args[last][:len(node.Args[last])-1]
			if len(node.Args[last]) == 0 {
				node.Args = node.Args[:last]
			}
			continueOnLF = true
			continue
		}
		break
	}

	macroName, macroArgs, err := ctx.parseAsMacro(&node)
	if err != nil {
		return node, err
	}
	if macroName != "" {
		node.Name = macroName
		node.Args = macroArgs
		node.Macro = true
	}

	if !node.Macro && !node.Snippet {
		if err := validateNodeName(node.Name); err != nil {
			return node, err
		}
	}

	return node, nil
}

func NodeErr(node Node, f string, args ...interface{}) error {
	if node.File == "" {
		return fmt.Errorf(f, args...)
	}
	return fmt.Errorf("%s:%d: %s", node.File, node.Line, fmt.Sprintf(f, args...))
}

func (ctx *parseContext) isSnippet(name string) (bool, string) {
	if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
		return true, name[1 : len(name)-1]
	}
	return false, ""
}

func (ctx *parseContext) parseAsMacro(node *Node) (macroName string, args []string, err error) {
	if !strings.HasPrefix(node.Name, "$(") {
		return "", nil, nil
	}
	if !strings.HasSuffix(node.Name, ")") {
		return "", nil, ctx.Err("macro name must end with )")
	}
	macroName = node.Name[2 : len(node.Name)-1]
	if len(node.Args) < 2 {
		return macroName, nil, ctx.Err("at least 2 arguments are required")
	}
	if node.Args[0] != "=" {
		return macroName, nil, ctx.Err("missing = in macro declaration")
	}
	return macroName, node.Args[1:], nil
}

// readNodes reads nodes from the currently parsed block.
//
// The lexer's cursor should point to the opening brace
// name arg0 arg1 {  #< this one
//   c0
//   c1
// }
//
// To stay consistent with readNode after this function returns the lexer's cursor points
// to the last token of the black (closing brace).
func (ctx *parseContext) readNodes() ([]Node, error) {
	// It is not 'var res []Node' because we want empty
	// but non-nil Children slice for empty braces.
	res := []Node{}

	if ctx.nesting > 255 {
		return res, ctx.Err("nesting limit reached")
	}

	ctx.nesting++

	var requireNewLine bool
	// This loop iterates over logical lines.
	// Here are some examples, '#' is placed before token where cursor is when
	// another iteration of this loop starts.
	//
	// #a
	// #a b
	// #a b {
	//   #ac aa
	// #}
	// #aa bbb bbb \
	//    ccc ccc
	// #a b { #ac aa }
	//
	// As can be seen by the latest example, sometimes such logical line might
	// not be terminated by an actual LF character and so this needs to be
	// handled carefully.
	//
	// Note that if the '}' is on the same physical line, it is currently
	// included as the part of the logical line, that is:
	// #a b { #ac aa }
	//        ^------- that's the logical line
	// #c d
	// ^--- that's the next logical line
	// This is handled by the "edge case" branch inside the loop.
	for {
		if requireNewLine {
			if !ctx.NextLine() {
				// If we can't advance cursor even without Line constraint -
				// that's EOF.
				if !ctx.Next() {
					return res, nil
				}
				return res, ctx.Err("newline is required after closing brace")
			}
		} else if !ctx.Next() {
			break
		}

		// name arg0 arg1 {
		//   c0
		//   c1
		// }
		// ^ called when we hit } on separate line,
		// This means block we hit end of our block.
		if ctx.Val() == "}" {
			ctx.nesting--
			// name arg0 arg1 { #<1
			// }   }
			// ^2  ^3
			//
			// After #1 ctx.nesting is incremented by ctx.nesting++ before this loop.
			// Then we advance cursor and hit }, we exit loop, ctx.nesting now becomes 0.
			// But then the parent block reader does the same when it hits #3 -
			// ctx.nesting becomes -1 and it fails.
			if ctx.nesting < 0 {
				return res, ctx.Err("unexpected }")
			}
			break
		}
		node, err := ctx.readNode()
		if err != nil {
			return res, err
		}
		requireNewLine = true

		shouldStop := false

		// name arg0 arg1 {
		//   c1 c2 }
		//         ^
		// Edge case, here we check if the last argument of the last node is a }
		// If it is - we stop as we hit the end of our block.
		if len(node.Args) != 0 && node.Args[len(node.Args)-1] == "}" {
			ctx.nesting--
			if ctx.nesting < 0 {
				return res, ctx.Err("unexpected }")
			}
			node.Args = node.Args[:len(node.Args)-1]
			shouldStop = true
		}

		if node.Macro {
			if ctx.nesting != 0 {
				return res, ctx.Err("macro declarations are only allowed at top-level")
			}

			// Macro declaration itself can contain macro references.
			if err := ctx.expandMacros(&node); err != nil {
				return res, err
			}

			// = sign is removed by parseAsMacro.
			// It also cuts $( and ) from name.
			ctx.macros[node.Name] = node.Args
			continue
		}
		if node.Snippet {
			if ctx.nesting != 0 {
				return res, ctx.Err("snippet declarations are only allowed at top-level")
			}
			if len(node.Args) != 0 {
				return res, ctx.Err("snippet declarations can't have arguments")
			}

			ctx.snippets[node.Name] = node.Children
			continue
		}

		if err := ctx.expandMacros(&node); err != nil {
			return res, err
		}

		res = append(res, node)
		if shouldStop {
			break
		}
	}
	return res, nil
}

func readTree(r io.Reader, location string, expansionDepth int) (nodes []Node, snips map[string][]Node, macros map[string][]string, err error) {
	ctx := parseContext{
		Dispenser:    lexer.NewDispenser(location, r),
		snippets:     make(map[string][]Node),
		macros:       map[string][]string{},
		nesting:      -1,
		fileLocation: location,
	}

	root := Node{}
	root.File = location
	root.Line = 1
	// Before parsing starts the lexer's cursor points to the non-existent
	// token before the first one. From readNodes viewpoint this is opening
	// brace so we don't break any requirements here.
	//
	// For the same reason we use -1 as a starting nesting. So readNodes
	// will see this as it is reading block at nesting level 0.
	root.Children, err = ctx.readNodes()
	if err != nil {
		return root.Children, ctx.snippets, ctx.macros, err
	}

	// There is no need to check ctx.nesting < 0 because it is checked by readNodes.
	if ctx.nesting > 0 {
		return root.Children, ctx.snippets, ctx.macros, ctx.Err("unexpected EOF when looking for }")
	}

	root, err = ctx.expandImports(root, expansionDepth)
	if err != nil {
		return root.Children, ctx.snippets, ctx.macros, err
	}

	return root.Children, ctx.snippets, ctx.macros, nil
}

func Read(r io.Reader, location string) (nodes []Node, err error) {
	nodes, _, _, err = readTree(r, location, 0)
	nodes = expandEnvironment(nodes)
	return
}
