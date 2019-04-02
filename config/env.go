package config

import (
	"os"
	"regexp"
	"strings"
)

func expandEnvironment(nodes []Node) []Node {
	replacer := buildEnvReplacer()
	newNodes := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Name = removeUnexpandedEnvvars(replacer.Replace(node.Name))
		for i, arg := range node.Args {
			node.Args[i] = removeUnexpandedEnvvars(replacer.Replace(arg))
		}
		node.Children = expandEnvironment(node.Children)
		newNodes = append(newNodes, node)
	}
	return newNodes
}

var unixEnvvarRe = regexp.MustCompile(`{\$([^\$]+)}`)
var winEnvvarRe = regexp.MustCompile(`{%([^%]+)%}`)

func removeUnexpandedEnvvars(s string) string {
	s = unixEnvvarRe.ReplaceAllString(s, "")
	s = winEnvvarRe.ReplaceAllString(s, "")
	return s
}

func buildEnvReplacer() *strings.Replacer {
	env := os.Environ()
	pairs := make([]string, 0, len(env)*4)
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := parts[1]

		pairs = append(pairs, "{%"+key+"%}", value)
		pairs = append(pairs, "{$"+key+"}", value)
	}
	return strings.NewReplacer(pairs...)
}
