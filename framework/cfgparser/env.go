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

	envMap := buildEnvMap()
	replacer := buildEnvReplacerFromMap(envMap)
	newNodes := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Name = removeUnexpandedEnvvars(replacer.Replace(node.Name))
		newArgs := make([]string, 0, len(node.Args))
		for _, arg := range node.Args {
			expArg := replacer.Replace(arg)
			if splitArgs, ok := handleEnvSplitWithMap(expArg, envMap); ok {
				newArgs = append(newArgs, splitArgs...)
			} else {
				newArgs = append(newArgs, removeUnexpandedEnvvars(expArg))
			}
		}
		node.Args = newArgs
		node.Children = expandEnvironment(node.Children)
		newNodes = append(newNodes, node)
	}
	return newNodes
}

var unixEnvvarRe = regexp.MustCompile(`{env:([^\$]+)}`)
var unixEnvSplitRe = regexp.MustCompile(`{env_split:([^\$]+)}`)

func removeUnexpandedEnvvars(s string) string {
	s = unixEnvvarRe.ReplaceAllString(s, "")
	return s
}

func buildEnvMap() map[string]string {
	env := os.Environ()
	envMap := make(map[string]string, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}

func buildEnvReplacerFromMap(envMap map[string]string) *strings.Replacer {
	pairs := make([]string, 0, len(envMap)*4)
	for key, value := range envMap {
		pairs = append(pairs, "{env:"+key+"}", value)
	}
	return strings.NewReplacer(pairs...)
}

func handleEnvSplitWithMap(arg string, envMap map[string]string) ([]string, bool) {
	matches := unixEnvSplitRe.FindStringSubmatch(arg)
	if len(matches) == 2 {
		value, exists := envMap[matches[1]]
		if !exists {
			return nil, false
		}
		splitValues := strings.Split(value, ",")
		return splitValues, true
	}
	return nil, false
}
