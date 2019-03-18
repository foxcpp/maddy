// config package provides set of utilities for configuration parsing
package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/caddy/caddyfile"
)

type CfgTreeNode struct {
	Name      string
	Args      []string
	Childrens []CfgTreeNode
	Snippet   bool
}

type parseContext struct {
	caddyfile.Dispenser
	parens   int
	snippets map[string][]CfgTreeNode

	fileLocation string
}

func (ctx *parseContext) readCfgNode() (CfgTreeNode, error) {
	node := CfgTreeNode{}
	if ctx.Val() == "{" {
		ctx.parens++
		return node, ctx.SyntaxErr("block header")
	}
	node.Name = ctx.Val()
	if ok, name := ctx.isSnippet(node.Name); ok {
		node.Name = name
		node.Snippet = true
	}

	for ctx.NextArg() {
		if ctx.Val() == "{" {
			ctx.parens++
			var err error
			node.Childrens, err = ctx.readCfgNodes()
			if err != nil {
				return node, err
			}
			break
		}
		if ctx.Val() == "}" {
			ctx.parens--
			if ctx.parens < 0 {
				return node, ctx.Err("Unexpected }")
			}

			return node, nil
		}
		node.Args = append(node.Args, ctx.Val())
	}

	return node, nil
}

func (ctx *parseContext) expandImports(node *CfgTreeNode, expansionDepth int) error {
	newChildrens := make([]CfgTreeNode, 0, len(node.Childrens))
	for _, child := range node.Childrens {
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
	node.Childrens = newChildrens
	return nil
}

func (ctx *parseContext) resolveImport(name string, depth int) ([]CfgTreeNode, error) {
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

func (ctx *parseContext) isSnippet(name string) (bool, string) {
	if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
		return true, name[1 : len(name)-1]
	}
	return false, ""
}

func (ctx *parseContext) readCfgNodes() ([]CfgTreeNode, error) {
	res := []CfgTreeNode{}
	if ctx.parens > 255 {
		return res, ctx.Err("Nesting limit reached")
	}
	for ctx.Next() {
		if ctx.Val() == "}" {
			ctx.parens--
			if ctx.parens < 0 {
				return res, ctx.Err("Unexpected }")
			}
			break
		}
		node, err := ctx.readCfgNode()
		if err != nil {
			return res, err
		}

		shouldStop := false
		if len(node.Args) != 0 && node.Args[len(node.Args)-1] == "}" {
			ctx.parens--
			if ctx.parens < 0 {
				return res, ctx.Err("Unexpected }")
			}
			node.Args = node.Args[:len(node.Args)-1]
			shouldStop = true
		}

		if node.Snippet {
			if ctx.parens != 0 {
				return res, ctx.Err("Snippet declarations are only allowed at top-level")
			}
			if len(node.Args) != 0 {
				return res, ctx.Err("Snippet declarations can't have arguments")
			}

			ctx.snippets[node.Name] = node.Childrens
			continue
		}

		if shouldStop {
			break
		}
		res = append(res, node)
	}
	return res, nil
}

func readCfgTree(r io.Reader, location string, depth int) (nodes []CfgTreeNode, snips map[string][]CfgTreeNode, err error) {
	ctx := parseContext{
		Dispenser:    caddyfile.NewDispenser(location, r),
		snippets:     make(map[string][]CfgTreeNode),
		fileLocation: location,
	}
	root := CfgTreeNode{}
	root.Childrens, err = ctx.readCfgNodes()
	if err != nil {
		return root.Childrens, ctx.snippets, err
	}
	if ctx.parens < 0 {
		return root.Childrens, ctx.snippets, ctx.Err("Unexpected }")
	}
	if ctx.parens > 0 {
		return root.Childrens, ctx.snippets, ctx.Err("Unexpected {")
	}

	if err := ctx.expandImports(&root, depth); err != nil {
		return root.Childrens, ctx.snippets, err
	}

	return root.Childrens, ctx.snippets, nil
}

func ReadCfgTree(r io.Reader, location string) (nodes []CfgTreeNode, err error) {
	nodes, _, err = readCfgTree(r, location, 1)
	return
}
