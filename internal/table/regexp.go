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

package table

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
)

type Regexp struct {
	modName  string
	instName string

	re           *regexp.Regexp
	replacements []string

	expandPlaceholders bool

	log *log.Logger
}

func NewRegexp(c *container.C, modName, instName string) (module.Module, error) {
	return &Regexp{
		modName:  modName,
		instName: instName,
		log:      c.DefaultLogger.Sublogger(modName),
	}, nil
}

func (r *Regexp) Configure(inlineArgs []string, cfg *config.Map) error {
	var (
		fullMatch       bool
		caseInsensitive bool
		oldExpand       *bool
	)
	cfg.Bool("full_match", false, true, &fullMatch)
	cfg.Bool("case_insensitive", false, true, &caseInsensitive)
	cfg.Bool("expand_placeholders", false, true, &r.expandPlaceholders)
	// Old misspelled name, kept for compatibility.
	cfg.Callback("expand_replaceholders", func(_ *config.Map, node config.Node) error {
		r.log.Msg("expand_replaceholders is deprecated, use expand_placeholders instead")
		if oldExpand != nil {
			return config.NodeErr(node, "duplicate directive: %s", node.Name)
		}
		if len(node.Children) != 0 {
			return config.NodeErr(node, "can't declare block here")
		}
		val := true
		switch len(node.Args) {
		case 0:
		case 1:
			b, err := config.ParseBool(node.Args[0])
			if err != nil {
				return config.NodeErr(node, "bool argument should be 'yes' or 'no'")
			}
			val = b
		default:
			return config.NodeErr(node, "expected exactly 1 argument")
		}
		oldExpand = &val
		return nil
	})
	if _, err := cfg.Process(); err != nil {
		return err
	}
	if oldExpand != nil {
		r.expandPlaceholders = *oldExpand
	}

	regex := inlineArgs[0]
	if len(inlineArgs) > 1 {
		r.replacements = inlineArgs[1:]
	}

	if fullMatch {
		if !strings.HasPrefix(regex, "^") {
			regex = "^" + regex
		}
		if !strings.HasSuffix(regex, "$") {
			regex = regex + "$"
		}
	}

	if caseInsensitive {
		regex = "(?i)" + regex
	}

	var err error
	r.re, err = regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("%s: %v", r.modName, err)
	}
	return nil
}

func (r *Regexp) Name() string {
	return r.modName
}

func (r *Regexp) InstanceName() string {
	return r.modName
}

func (r *Regexp) LookupMulti(_ context.Context, key string) ([]string, error) {
	matches := r.re.FindStringSubmatchIndex(key)
	if matches == nil {
		return []string{}, nil
	}

	result := []string{}
	for _, replacement := range r.replacements {
		if !r.expandPlaceholders {
			result = append(result, replacement)
		} else {
			result = append(result, string(r.re.ExpandString([]byte{}, replacement, key, matches)))
		}
	}
	return result, nil
}

func (r *Regexp) Lookup(ctx context.Context, key string) (string, bool, error) {
	newVal, err := r.LookupMulti(ctx, key)
	if err != nil {
		return "", false, err
	}
	if len(newVal) == 0 {
		return "", false, nil
	}

	return newVal[0], true, nil
}

func init() {
	modules.Register("table.regexp", NewRegexp)
}
