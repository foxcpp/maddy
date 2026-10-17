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
	"errors"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

// addHeader is a simple module that adds a header to the message
type addHeader struct {
	modName  string
	instName string

	headerName  string
	headerValue string
}

func NewAddHeader(modName, instName string) (module.Module, error) {
	r := addHeader{
		modName:  modName,
		instName: instName,
	}

	return &r, nil
}

func (m *addHeader) Configure(inlineArgs []string, cfg *config.Map) error {
	if len(inlineArgs) != 2 {
		return errors.New("modify.add_header: at least two arguments required")
	}
	m.headerName = inlineArgs[0]
	m.headerValue = inlineArgs[1]
	return nil
}

func (r addHeader) Name() string {
	return r.modName
}

func (r addHeader) InstanceName() string {
	return r.instName
}

func (r addHeader) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return r, nil
}

func (r addHeader) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	return mailFrom, nil
}

func (r addHeader) RewriteRcpt(ctx context.Context, rcptTo string) ([]string, error) {
	return []string{rcptTo}, nil
}

func (r addHeader) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	h.Add(r.headerName, r.headerValue)
	return nil
}

func (r addHeader) Close() error {
	return nil
}

func init() {
	module.Register("modify.add_header", NewAddHeader)
}
