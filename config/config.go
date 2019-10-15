package config

import (
	"fmt"

	"github.com/foxcpp/maddy/config/parser"
)

type (
	Node = parser.Node
)

func NodeErr(node *Node, f string, args ...interface{}) error {
	if node.File == "" {
		return fmt.Errorf(f, args...)
	}
	return fmt.Errorf("%s:%d: %s", node.File, node.Line, fmt.Sprintf(f, args...))
}
