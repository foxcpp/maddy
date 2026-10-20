/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2025 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

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

/*
Package ignore_error implements a delivery target that wraps another target and
turns any delivery error into a logged warning instead of propagating it.

It is meant for non-critical deliver_to copies (e.g. push notifications) where a
failure should not affect the rest of the pipeline. The wrapped target is given
inline:

	deliver_to ignore_error smtp tcp://127.0.0.1:2525

Any configuration block attached to the directive belongs to the wrapped target
and is forwarded as-is.
*/
package ignore_error

import (
	"context"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	"github.com/foxcpp/maddy/framework/container"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/framework/module/modules"
	"github.com/foxcpp/maddy/internal/target"
)

const modName = "target.ignore_error"

type Target struct {
	instName string
	log      *log.Logger
	wrapped  module.DeliveryTarget
}

func New(c *container.C, _, instName string) (module.Module, error) {
	return &Target{
		instName: instName,
		log:      c.DefaultLogger.Sublogger(modName),
	}, nil
}

func (t *Target) Configure(inlineArgs []string, cfg *config.Map) error {
	// The wrapped target is defined inline, so its name and arguments arrive as
	// inlineArgs and any attached block belongs to it. Forward both to the
	// module loader instead of consuming them here.
	return modconfig.ModuleFromNode("target", inlineArgs, cfg.Block, cfg.Globals, &t.wrapped)
}

func (t *Target) Name() string {
	return modName
}

func (t *Target) InstanceName() string {
	return t.instName
}

func (t *Target) StartDelivery(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	d := &delivery{
		log: target.DeliveryLogger(t.log, msgMeta),
	}

	wrapped, err := t.wrapped.StartDelivery(ctx, msgMeta, mailFrom)
	if err != nil {
		// Leave d.wrapped nil so the rest of the delivery is a no-op.
		d.log.Error("ignored error from wrapped target", err)
		return d, nil
	}
	d.wrapped = wrapped
	return d, nil
}

type delivery struct {
	wrapped module.Delivery
	log     *log.Logger
}

func (d *delivery) AddRcpt(ctx context.Context, rcptTo string, opts smtp.RcptOptions) error {
	if d.wrapped == nil {
		return nil
	}
	if err := d.wrapped.AddRcpt(ctx, rcptTo, opts); err != nil {
		d.log.Error("ignored error from wrapped target", err, "rcpt", rcptTo)
	}
	return nil
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	if d.wrapped == nil {
		return nil
	}
	if err := d.wrapped.Body(ctx, header, body); err != nil {
		d.log.Error("ignored error from wrapped target", err)
	}
	return nil
}

func (d *delivery) Abort(ctx context.Context) error {
	if d.wrapped == nil {
		return nil
	}
	if err := d.wrapped.Abort(ctx); err != nil {
		d.log.Error("ignored error from wrapped target", err)
	}
	return nil
}

func (d *delivery) Commit(ctx context.Context) error {
	if d.wrapped == nil {
		return nil
	}
	if err := d.wrapped.Commit(ctx); err != nil {
		d.log.Error("ignored error from wrapped target", err)
	}
	return nil
}

func init() {
	modules.Register(modName, New)
}
