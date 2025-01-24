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
	"path/filepath"
	"regexp"
	"strings"
)

func (ctx *parseContext) expandImports(node Node, expansionDepth int) (Node, error) {
	// Leave nil value as is because it is used as non-existent block indicator
	// (vs empty slice - empty block).
	if node.Children == nil {
		return node, nil
	}

	newChildrens := make([]Node, 0, len(node.Children))
	containsImports := false
	for _, child := range node.Children {
		child, err := ctx.expandImports(child, expansionDepth+1)
		if err != nil {
			return node, err
		}

		if child.Name == "import" {
			// We check it here instead of function start so we can
			// use line information from import directive that is likely
			// caused this error.
			if expansionDepth > 255 {
				return node, NodeErr(child, "hit import expansion limit")
			}

			containsImports = true
			if len(child.Args) != 1 {
				return node, ctx.Err("import directive requires exactly 1 argument")
			}

			subtree, err := ctx.resolveImport(child, child.Args[0], expansionDepth)
			if err != nil {
				return node, err
			}

			newChildrens = append(newChildrens, subtree...)
		} else {
			newChildrens = append(newChildrens, child)
		}
	}
	node.Children = newChildrens

	// We need to do another pass to expand any imports added by snippets we
	// just expanded.
	if containsImports {
		return ctx.expandImports(node, expansionDepth+1)
	}

	return node, nil
}

func (ctx *parseContext) resolveImport(node Node, name string, expansionDepth int) ([]Node, error) {
	if subtree, ok := ctx.snippets[name]; ok {
		return subtree, nil
	}

	file := name
	if !filepath.IsAbs(name) {
		file = filepath.Join(filepath.Dir(ctx.fileLocation), name)
	}
	src, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			src, err = os.Open(file + ".conf")
			if err != nil {
				if os.IsNotExist(err) {
					return nil, NodeErr(node, "unknown import: %s", name)
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	nodes, snips, macros, err := readTree(src, file, expansionDepth+1)
	if err != nil {
		return nodes, err
	}
	for k, v := range snips {
		ctx.snippets[k] = v
	}
	for k, v := range macros {
		ctx.macros[k] = v
	}

	return nodes, nil
}

func (ctx *parseContext) expandMacros(node *Node) error {
	if strings.HasPrefix(node.Name, "$(") && strings.HasSuffix(node.Name, ")") {
		return ctx.Err("can't use macro argument as directive name")
	}

	newArgs := make([]string, 0, len(node.Args))
	for _, arg := range node.Args {
		if !strings.HasPrefix(arg, "$(") || !strings.HasSuffix(arg, ")") {
			if strings.Contains(arg, "$(") && strings.Contains(arg, ")") {
				var err error
				arg, err = ctx.expandSingleValueMacro(arg)
				if err != nil {
					return err
				}
			}

			newArgs = append(newArgs, arg)
			continue
		}

		macroName := arg[2 : len(arg)-1]
		replacement, ok := ctx.macros[macroName]
		if !ok {
			// Undefined macros are expanded to zero arguments.
			continue
		}

		newArgs = append(newArgs, replacement...)
	}
	node.Args = newArgs

	if node.Children != nil {
		for i := range node.Children {
			if err := ctx.expandMacros(&node.Children[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

var macroRe = regexp.MustCompile(`\$\(([^\$]+)\)`)

func (ctx *parseContext) expandSingleValueMacro(arg string) (string, error) {
	matches := macroRe.FindAllStringSubmatch(arg, -1)
	for _, match := range matches {
		macroName := match[1]
		if len(ctx.macros[macroName]) > 1 {
			return "", ctx.Err("can't expand macro with multiple arguments inside a string")
		}

		var value string
		if ctx.macros[macroName] != nil {
			// Macros have at least one argument.
			value = ctx.macros[macroName][0]
		}

		arg = strings.Replace(arg, "$("+macroName+")", value, -1)
	}

	return arg, nil
}
