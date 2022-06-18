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

package imap_filter

import (
	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

// Group wraps multiple modifiers and runs them serially.
//
// It is also registered as a module under 'modifiers' name and acts as a
// module group.
type Group struct {
	instName string
	Filters  []module.IMAPFilter
	log      log.Logger
}

func NewGroup(_, instName string, _, _ []string) (module.Module, error) {
	return &Group{
		instName: instName,
		log:      log.Logger{Name: "imap_filters", Debug: log.DefaultLogger.Debug},
	}, nil
}

func (g *Group) IMAPFilter(accountName string, rcptTo string, meta *module.MsgMetadata, hdr textproto.Header, body buffer.Buffer) (folder string, flags []string, err error) {
	if g == nil {
		return "", nil, nil
	}
	var (
		finalFolder string
		finalFlags  = make([]string, 0, len(g.Filters))
	)
	for _, f := range g.Filters {
		folder, flags, err := f.IMAPFilter(accountName, rcptTo, meta, hdr, body)
		if err != nil {
			g.log.Error("IMAP filter failed", err)
			continue
		}
		if folder != "" && finalFolder == "" {
			finalFolder = folder
		}
		finalFlags = append(finalFlags, flags...)
	}
	return finalFolder, finalFlags, nil
}

func (g *Group) Init(cfg *config.Map) error {
	for _, node := range cfg.Block.Children {
		mod, err := modconfig.IMAPFilter(cfg.Globals, append([]string{node.Name}, node.Args...), node)
		if err != nil {
			return err
		}

		g.Filters = append(g.Filters, mod)
	}

	return nil
}

func (g *Group) Name() string {
	return "modifiers"
}

func (g *Group) InstanceName() string {
	return g.instName
}

func init() {
	module.Register("imap_filters", NewGroup)
}
