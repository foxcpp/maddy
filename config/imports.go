package config

import (
	"os"
	"path/filepath"
)

func (ctx *parseContext) expandImports(node *Node, expansionDepth int) error {
	newChildrens := make([]Node, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Name == "import" {
			if len(child.Args) != 1 {
				return ctx.Err("import directive requires exactly 1 argument")
			}

			subtree, err := ctx.resolveImport(child.Args[0], expansionDepth)
			if err != nil {
				return err
			}

			newChildrens = append(newChildrens, subtree...)
		} else {
			if err := ctx.expandImports(&child, expansionDepth+1); err != nil {
				return err
			}

			newChildrens = append(newChildrens, child)
		}
	}
	node.Children = newChildrens
	return nil
}

func (ctx *parseContext) resolveImport(name string, depth int) ([]Node, error) {
	if subtree, ok := ctx.snippets[name]; ok {
		return subtree, nil
	}

	file := filepath.Join(filepath.Dir(ctx.fileLocation), name)
	src, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ctx.Err("Unknown import: " + name)
		}
		return nil, err
	}
	nodes, snips, err := readCfgTree(src, file, depth+1)
	if err != nil {
		return nodes, err
	}
	for k, v := range snips {
		ctx.snippets[k] = v
	}

	return nodes, nil
}
