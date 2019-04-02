// Package config provides set of utilities for configuration parsing.
package config

import (
	"fmt"
	"io"
	"strings"

	"github.com/mholt/caddy/caddyfile"
)

// name arg0 arg1 {
//  children0
//  children1
// }
type Node struct {
	// First string at node's line.
	Name string
	// Any strings placed after node name.
	Args []string

	// If node is a block - all nodes inside the block. Can be nil.
	Children []Node

	// Whether current parsed node is a snippet. Always false for all nodes
	// returned from Read because snippets are expanded before it returns.
	Snippet bool

	File string
	Line int
}

type parseContext struct {
	caddyfile.Dispenser
	nesting  int
	snippets map[string][]Node

	fileLocation string
}

// readNode reads node starting at current token pointed by lexer's
// cursor (it should point to node name).
//
// After readNode returns lexer's cursor will point to last token of parsed
// Node. This ensures predicatable cursor location independently of EOF state.
// Thus code reading multiple nodes should call readNode then manually
// advance lexer cursor (ctx.Next) and either call readNode again or stop
// because cursor hit EOF.
//
// readNode calls readNodes if currently parsed node is a block.
// ctx.nesting is incremented call.
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

	for ctx.NextArg() {
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

	return node, nil
}

func (ctx *parseContext) NodeErr(node *Node, message string) error {
	return fmt.Errorf("%s:%d - %s", node.File, node.Line, message)
}

func (ctx *parseContext) isSnippet(name string) (bool, string) {
	if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
		return true, name[1 : len(name)-1]
	}
	return false, ""
}

// readNodes reads nodes from currently parsed block.
//
// Lexer's cursor should point to opening brace
// name arg0 arg1 {  #< this one
//   c0
//   c1
// }
//
// To stay consistent with readNode after this function returns lexer's cursor pointer
// to the last token of the black (closing brace).
func (ctx *parseContext) readNodes() ([]Node, error) {
	res := []Node{}

	// Refuse to continue is nesting is too big.
	if ctx.nesting > 255 {
		return res, ctx.Err("Nesting limit reached")
	}

	ctx.nesting++

	for ctx.Next() {
		// name arg0 arg1 {
		//   c0
		//   c1
		// }
		// ^ called when we hit } on separate line,
		// This means block we are supposed to parse ended and we should stop.
		if ctx.Val() == "}" {
			ctx.nesting--
			// name arg0 arg1 { #<1
			// }   }
			// ^2  ^3
			//
			// After #1 ctx.nesting is incremented by ctx.nesting++ before this loop.
			// Then we advance cursor and hit }, we exit loop, ctx.nesting now becomes 0.
			// But then parent block reader does the same when it hits #3 -
			// ctx.nesting becomes -1 and it fails.
			if ctx.nesting < 0 {
				return res, ctx.Err("Unexpected }")
			}
			break
		}
		node, err := ctx.readNode()
		if err != nil {
			return res, err
		}

		shouldStop := false

		// name arg0 arg1 {
		//   c1 c2 }
		//         ^
		// Edgy case, here we check if last argument of last node is a }
		// If it is - we stop as we hit end of our block.
		if len(node.Args) != 0 && node.Args[len(node.Args)-1] == "}" {
			ctx.nesting--
			if ctx.nesting < 0 {
				return res, ctx.Err("Unexpected }")
			}
			node.Args = node.Args[:len(node.Args)-1]
			shouldStop = true
		}

		if node.Snippet {
			if ctx.nesting != 0 {
				return res, ctx.Err("Snippet declarations are only allowed at top-level")
			}
			if len(node.Args) != 0 {
				return res, ctx.Err("Snippet declarations can't have arguments")
			}

			ctx.snippets[node.Name] = node.Children
			continue
		}

		res = append(res, node)
		if shouldStop {
			break
		}
	}
	return res, nil
}

func readTree(r io.Reader, location string, expansionDepth int) (nodes []Node, snips map[string][]Node, err error) {
	ctx := parseContext{
		Dispenser:    caddyfile.NewDispenser(location, r),
		snippets:     make(map[string][]Node),
		nesting:      -1,
		fileLocation: location,
	}

	root := Node{}
	root.File = location
	root.Line = 1
	// Before parsing starts lexer's cursor points to the non-existent token before
	// first one. From readNodes viewpoint this is opening brace so we
	// don't break any requirements here.
	//
	// For the same reason we use -1 as a starting nesting. So readNodes
	// will see this as it is reading block at nesting level 0.
	root.Children, err = ctx.readNodes()
	if err != nil {
		return root.Children, ctx.snippets, err
	}

	// There is no need to check ctx.nesting < 0 because it is checked by readNodes.
	if ctx.nesting > 0 {
		return root.Children, ctx.snippets, ctx.Err("Unexpected {")
	}

	if err := ctx.expandImports(&root, expansionDepth); err != nil {
		return root.Children, ctx.snippets, err
	}

	return root.Children, ctx.snippets, nil
}

func Read(r io.Reader, location string) (nodes []Node, err error) {
	nodes, _, err = readTree(r, location, 0)
	nodes = expandEnvironment(nodes)
	return
}
