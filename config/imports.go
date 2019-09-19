package config

import (
	"os"
	"path/filepath"
	"strings"
)

func (ctx *parseContext) expandImports(node *Node, expansionDepth int) error {
	// Leave nil value as is because it is used as non-existent block indicator
	// (vs empty slice - empty block).
	if node.Children == nil {
		return nil
	}

	newChildrens := make([]Node, 0, len(node.Children))
	containsImports := false
	for _, child := range node.Children {
		if child.Name == "import" {
			// We check it here instead of function start so we can
			// use line information from import directive that is likely
			// caused this error.
			if expansionDepth > 255 {
				return NodeErr(&child, "hit import expansion limit")
			}

			containsImports = true
			if len(child.Args) != 1 {
				return ctx.Err("import directive requires exactly 1 argument")
			}

			subtree, err := ctx.resolveImport(&child, child.Args[0], expansionDepth)
			if err != nil {
				return err
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
		if err := ctx.expandImports(node, expansionDepth+1); err != nil {
			return err
		}
	}

	return nil
}

func (ctx *parseContext) resolveImport(node *Node, name string, expansionDepth int) ([]Node, error) {
	if subtree, ok := ctx.snippets[name]; ok {
		return subtree, nil
	}

	file := filepath.Join(filepath.Dir(ctx.fileLocation), name)
	src, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NodeErr(node, "unknown import: "+name)
		}
		return nil, err
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
