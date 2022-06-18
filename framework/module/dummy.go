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

package module

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
)

// Dummy is a struct that implements PlainAuth and DeliveryTarget
// interfaces but does nothing. Useful for testing.
//
// It is always registered under the 'dummy' name and can be used in both tests
// and the actual server code (but the latter is kinda pointless).
type Dummy struct{ instName string }

func (d *Dummy) AuthPlain(username, _ string) error {
	return nil
}

func (d *Dummy) Lookup(_ context.Context, _ string) (string, bool, error) {
	return "", false, nil
}

func (d *Dummy) LookupMulti(_ context.Context, _ string) ([]string, error) {
	return []string{""}, nil
}

func (d *Dummy) Name() string {
	return "dummy"
}

func (d *Dummy) InstanceName() string {
	return d.instName
}

func (d *Dummy) Init(_ *config.Map) error {
	return nil
}

func (d *Dummy) Start(ctx context.Context, msgMeta *MsgMetadata, mailFrom string) (Delivery, error) {
	return dummyDelivery{}, nil
}

type dummyDelivery struct{}

func (dd dummyDelivery) AddRcpt(ctx context.Context, to string) error {
	return nil
}

func (dd dummyDelivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	return nil
}

func (dd dummyDelivery) Abort(ctx context.Context) error {
	return nil
}

func (dd dummyDelivery) Commit(ctx context.Context) error {
	return nil
}

func init() {
	Register("dummy", func(_, instName string, _, _ []string) (Module, error) {
		return &Dummy{instName: instName}, nil
	})
}
