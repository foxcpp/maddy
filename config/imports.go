package config

import (
	"os"
	"path/filepath"
)

func (ctx *parseContext) expandImports(node *Node, expansionDepth int) error {
	newChildrens := make([]Node, 0, len(node.Children))
	containsImports := false
	for _, child := range node.Children {
		if child.Name == "import" {
			// We check it here instead of function start so we can
			// use line information from import directive that is likely
			// caused this error.
			if expansionDepth > 255 {
				return ctx.NodeErr(&child, "hit import expansion limit")
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
			return nil, ctx.NodeErr(node, "unknown import: "+name)
		}
		return nil, err
	}
	nodes, snips, err := readCfgTree(src, file, expansionDepth+1)
	if err != nil {
		return nodes, err
	}
	for k, v := range snips {
		ctx.snippets[k] = v
	}

	return nodes, nil
}
