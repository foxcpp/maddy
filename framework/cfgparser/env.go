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
	"regexp"
	"strings"
)

func expandEnvironment(nodes []Node) []Node {
	// If nodes is nil - don't replace with empty slice, as nil indicates "no
	// block".
	if nodes == nil {
		return nil
	}

	replacer := buildEnvReplacer()
	newNodes := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Name = removeUnexpandedEnvvars(replacer.Replace(node.Name))
		newArgs := make([]string, 0, len(node.Args))
		for _, arg := range node.Args {
			newArgs = append(newArgs, removeUnexpandedEnvvars(replacer.Replace(arg)))
		}
		node.Args = newArgs
		node.Children = expandEnvironment(node.Children)
		newNodes = append(newNodes, node)
	}
	return newNodes
}

var unixEnvvarRe = regexp.MustCompile(`{env:([^\$]+)}`)

func removeUnexpandedEnvvars(s string) string {
	s = unixEnvvarRe.ReplaceAllString(s, "")
	return s
}

func buildEnvReplacer() *strings.Replacer {
	env := os.Environ()
	pairs := make([]string, 0, len(env)*4)
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := parts[1]

		pairs = append(pairs, "{env:"+key+"}", value)
	}
	return strings.NewReplacer(pairs...)
}
