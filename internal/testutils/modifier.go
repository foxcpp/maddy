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

package testutils

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Modifier struct {
	InstName string

	InitErr     error
	MailFromErr error
	RcptToErr   error
	BodyErr     error

	MailFrom map[string]string
	RcptTo   map[string][]string
	AddHdr   textproto.Header

	UnclosedStates int
}

func (m Modifier) Init(*config.Map) error {
	return nil
}

func (m Modifier) Name() string {
	return "test_modifier"
}

func (m Modifier) InstanceName() string {
	return m.InstName
}

type modifierState struct {
	m *Modifier
}

func (m Modifier) ModStateForMsg(ctx context.Context, msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	if m.InitErr != nil {
		return nil, m.InitErr
	}

	m.UnclosedStates++
	return modifierState{&m}, nil
}

func (ms modifierState) RewriteSender(ctx context.Context, mailFrom string) (string, error) {
	if ms.m.MailFromErr != nil {
		return "", ms.m.MailFromErr
	}
	if ms.m.MailFrom == nil {
		return mailFrom, nil
	}

	newMailFrom, ok := ms.m.MailFrom[mailFrom]
	if ok {
		return newMailFrom, nil
	}
	return mailFrom, nil
}

func (ms modifierState) RewriteRcpt(ctx context.Context, rcptTo string) ([]string, error) {
	if ms.m.RcptToErr != nil {
		return []string{""}, ms.m.RcptToErr
	}

	if ms.m.RcptTo == nil {
		return []string{rcptTo}, nil
	}

	newRcptTo, ok := ms.m.RcptTo[rcptTo]
	if ok {
		return newRcptTo, nil
	}
	return []string{rcptTo}, nil
}

func (ms modifierState) RewriteBody(ctx context.Context, h *textproto.Header, body buffer.Buffer) error {
	if ms.m.BodyErr != nil {
		return ms.m.BodyErr
	}

	for field := ms.m.AddHdr.Fields(); field.Next(); {
		h.Add(field.Key(), field.Value())
	}
	return nil
}

func (ms modifierState) Close() error {
	ms.m.UnclosedStates--
	return nil
}

func init() {
	module.Register("test_modifier", func(_, _ string, _, _ []string) (module.Module, error) {
		return &Modifier{}, nil
	})
	module.RegisterInstance(&Modifier{}, nil)
}
