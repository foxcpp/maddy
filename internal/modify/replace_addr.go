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

package modify

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/module"
)

// replaceAddr is a simple module that replaces matching sender (or recipient) address
// in messages using module.Table implementation.
//
// If created with modName = "replace_sender", it will change sender address.
// If created with modName = "replace_rcpt", it will change recipient addresses.
type replaceAddr struct {
	modName    string
	instName   string
	inlineArgs []string

	replaceSender bool
	replaceRcpt   bool
	table         module.Table
}

func NewReplaceAddr(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	r := replaceAddr{
		modName:       modName,
		instName:      instName,
		inlineArgs:    inlineArgs,
		replaceSender: modName == "replace_sender",
		replaceRcpt:   modName == "replace_rcpt",
	}

	return &r, nil
}

func (r *replaceAddr) Init(cfg *config.Map) error {
	return modconfig.ModuleFromNode("table", r.inlineArgs, cfg.Block, cfg.Globals, &r.table)
}

func (r replaceAddr) Name() string {
	return r.modName
}

func (r replaceAddr) InstanceName() string {
	return r.instName
}

func (r replaceAddr) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return r, nil
}

func (r replaceAddr) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	if r.replaceSender {
		return r.rewrite(mailFrom)
	}
	return mailFrom, nil
}

func (r replaceAddr) RewriteRcpt(ctx context.Context, rcptTo string) (string, error) {
	if r.replaceRcpt {
		return r.rewrite(rcptTo)
	}
	return rcptTo, nil
}

func (r replaceAddr) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	return nil
}

func (r replaceAddr) Close() error {
	return nil
}

func (r replaceAddr) rewrite(val string) (string, error) {
	normAddr, err := address.ForLookup(val)
	if err != nil {
		return val, fmt.Errorf("malformed address: %v", err)
	}

	replacement, ok, err := r.table.Lookup(normAddr)
	if err != nil {
		return val, err
	}
	if ok {
		if !address.Valid(replacement) {
			return "", fmt.Errorf("refusing to replace recipient with the invalid address %s", replacement)
		}
		return replacement, nil
	}

	mbox, domain, err := address.Split(normAddr)
	if err != nil {
		// If we have malformed address here, something is really wrong, but let's
		// ignore it silently then anyway.
		return val, nil
	}

	// mbox is already normalized, since it is a part of address.ForLookup
	// result.
	replacement, ok, err = r.table.Lookup(mbox)
	if err != nil {
		return val, err
	}
	if ok {
		if strings.Contains(replacement, "@") && !strings.HasPrefix(replacement, `"`) && !strings.HasSuffix(replacement, `"`) {
			if !address.Valid(replacement) {
				return "", fmt.Errorf("refusing to replace recipient with invalid address %s", replacement)
			}
			return replacement, nil
		}
		return replacement + "@" + domain, nil
	}

	return val, nil
}

func init() {
	module.Register("modify.replace_sender", NewReplaceAddr)
	module.RegisterDeprecated("replace_sender", "modify.replace_sender", NewReplaceAddr)
	module.Register("modify.replace_rcpt", NewReplaceAddr)
	module.RegisterDeprecated("replace_rcpt", "modify.replace_rcpt", NewReplaceAddr)
}
