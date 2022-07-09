//go:build ignore
// +build ignore

// Copy that file into target/ subdirectory.

package target_name

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2021 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

const modName = "target.target_name"

type Target struct {
	instName string
	log      log.Logger
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	// If wanted, extract any values from inlineArgs (these values:
	// deliver_to target_name ARG1 ARG2 { ... }

	return &Target{
		instName: instName,
		log:      log.Logger{Name: instName},
	}, nil
}

func (t *Target) Init(cfg *config.Map) error {
	cfg.Bool("debug", true, false, &t.log.Debug)

	// Read any config directives into Target variables here.

	if _, err := cfg.Process(); err != nil {
		return err
	}

	// Finish setup using obtained values.

	return nil
}

func (t *Target) Name() string {
	return modName
}

func (t *Target) InstanceName() string {
	return t.instName
}

// If it necessary to have any server shutdown cleanup - implement Close.

func (t *Target) Close() error {
	return nil
}

type delivery struct {
	t        *Target
	mailFrom string
	log      log.Logger
	msgMeta  *module.MsgMetadata
}

/*
See module.DeliveryTarget and module.Delivery docs for details on each method.
*/

func (t *Target) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	return &delivery{
		t:        t,
		mailFrom: mailFrom,
		log:      DeliveryLogger(t.log, msgMeta),
		msgMeta:  msgMeta,
	}, nil
}

func (d *delivery) AddRcpt(ctx context.Context, rcptTo string) error {
	// Corresponds to SMTP RCPT command.
	panic("implement me")
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	// Corresponds to SMTP DATA command.
	panic("implement me")
}

/*
If Body call can fail partially (either success or fail for each recipient passed to AddRcpt)
- implement BodyNonAtomic and signal status for each recipient using StatusCollector callback.

func (d *delivery) BodyNonAtomic(ctx context.Context, sc module.StatusCollector, header textproto.Header, body buffer.Buffer) {

}
*/

func (d *delivery) Abort(ctx context.Context) error {
	panic("implement me")
}

func (d *delivery) Commit(ctx context.Context) error {
	panic("implement me")
}

func init() {
	module.Register(modName, New)
}
