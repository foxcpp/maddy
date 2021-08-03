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

// Package smtp_downstream provides target.smtp module that implements
// transparent forwarding or messages to configured list of SMTP servers.
//
// Like remote module, this implementation doesn't handle atomic
// delivery properly since it is impossible to do with SMTP protocol
//
// Interfaces implemented:
// - module.DeliveryTarget
package smtp_downstream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime/trace"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/smtpconn"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
)

type Downstream struct {
	modName    string
	instName   string
	lmtp       bool
	targetsArg []string

	requireTLS      bool
	attemptStartTLS bool
	hostname        string
	endpoints       []config.Endpoint
	saslFactory     saslClientFactory
	tlsConfig       tls.Config

	connectTimeout    time.Duration
	commandTimeout    time.Duration
	submissionTimeout time.Duration

	log log.Logger
}

func (u *Downstream) moduleError(err error) error {
	if err == nil {
		return nil
	}

	return exterrors.WithFields(err, map[string]interface{}{
		"target": u.modName,
	})
}

func NewDownstream(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Downstream{
		modName:    modName,
		instName:   instName,
		lmtp:       modName == "target.lmtp" || modName == "lmtp_downstream", /* compatibility with 0.3 configs */
		targetsArg: inlineArgs,
		log:        log.Logger{Name: modName},
	}, nil
}

func (u *Downstream) Init(cfg *config.Map) error {
	var targetsArg []string
	cfg.Bool("debug", true, false, &u.log.Debug)
	cfg.Bool("require_tls", false, false, &u.requireTLS)
	cfg.Bool("attempt_starttls", false, !u.lmtp, &u.attemptStartTLS)
	cfg.String("hostname", true, true, "", &u.hostname)
	cfg.StringList("targets", false, false, nil, &targetsArg)
	cfg.Custom("auth", false, false, func() (interface{}, error) {
		return nil, nil
	}, saslAuthDirective, &u.saslFactory)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return tls.Config{}, nil
	}, tls2.TLSClientBlock, &u.tlsConfig)
	cfg.Duration("connect_timeout", false, false, 5*time.Minute, &u.connectTimeout)
	cfg.Duration("command_timeout", false, false, 5*time.Minute, &u.commandTimeout)
	cfg.Duration("submission_timeout", false, false, 5*time.Minute, &u.submissionTimeout)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	var err error
	u.hostname, err = idna.ToASCII(u.hostname)
	if err != nil {
		return fmt.Errorf("%s: cannot represent the hostname as an A-label name: %w", u.modName, err)
	}

	u.targetsArg = append(u.targetsArg, targetsArg...)
	for _, tgt := range u.targetsArg {
		endp, err := config.ParseEndpoint(tgt)
		if err != nil {
			return err
		}

		u.endpoints = append(u.endpoints, endp)
	}

	if len(u.endpoints) == 0 {
		return fmt.Errorf("%s: at least one target endpoint is required", u.modName)
	}

	return nil
}

func (u *Downstream) Name() string {
	return u.modName
}

func (u *Downstream) InstanceName() string {
	return u.instName
}

type delivery struct {
	u   *Downstream
	log log.Logger

	msgMeta  *module.MsgMetadata
	mailFrom string
	rcpts    []string

	conn *smtpconn.C
}

// lmtpDelivery implements module.PartialDelivery
type lmtpDelivery struct {
	*delivery
}

func (u *Downstream) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	defer trace.StartRegion(ctx, "target.smtp/Start").End()

	d := &delivery{
		u:        u,
		log:      target.DeliveryLogger(u.log, msgMeta),
		msgMeta:  msgMeta,
		mailFrom: mailFrom,
	}
	if err := d.connect(ctx); err != nil {
		return nil, err
	}

	if err := d.conn.Mail(ctx, mailFrom, msgMeta.SMTPOpts); err != nil {
		d.conn.Close()
		return nil, err
	}

	if u.lmtp {
		return &lmtpDelivery{delivery: d}, nil
	}

	return d, nil
}

func (d *delivery) connect(ctx context.Context) error {
	// TODO: Review possibility of connection pooling here.
	var lastErr error

	conn := smtpconn.New()
	conn.Log = d.log
	conn.Hostname = d.u.hostname
	conn.AddrInSMTPMsg = false
	if d.u.connectTimeout != 0 {
		conn.ConnectTimeout = d.u.connectTimeout
	}
	if d.u.commandTimeout != 0 {
		conn.CommandTimeout = d.u.commandTimeout
	}
	if d.u.submissionTimeout != 0 {
		conn.SubmissionTimeout = d.u.submissionTimeout
	}

	for _, endp := range d.u.endpoints {
		var (
			didTLS bool
			err    error
		)
		if d.u.lmtp {
			didTLS, err = conn.ConnectLMTP(ctx, endp, d.u.attemptStartTLS, &d.u.tlsConfig)
		} else {
			didTLS, err = conn.Connect(ctx, endp, d.u.attemptStartTLS, &d.u.tlsConfig)
		}
		if err != nil {
			if len(d.u.endpoints) != 1 {
				d.log.Msg("connect error", err, "downstream_server", net.JoinHostPort(endp.Host, endp.Port))
			}
			lastErr = err
			continue
		}

		d.log.DebugMsg("connected", "downstream_server", conn.ServerName())

		if !didTLS && d.u.requireTLS {
			conn.Close()
			lastErr = errors.New("TLS is required, but unsupported by downstream")
			continue
		}

		lastErr = nil
		break
	}
	if lastErr != nil {
		return d.u.moduleError(lastErr)
	}

	if d.u.saslFactory != nil {
		saslClient, err := d.u.saslFactory(d.msgMeta)
		if err != nil {
			conn.Close()
			return err
		}

		if err := conn.Client().Auth(saslClient); err != nil {
			conn.Close()
			return err
		}
	}

	d.conn = conn

	return nil
}

func (d *delivery) AddRcpt(ctx context.Context, rcptTo string) error {
	err := d.conn.Rcpt(ctx, rcptTo)
	if err != nil {
		return d.u.moduleError(err)
	}

	d.rcpts = append(d.rcpts, rcptTo)
	return nil
}

func (d *delivery) Body(ctx context.Context, header textproto.Header, body buffer.Buffer) error {
	r, err := body.Open()
	if err != nil {
		return exterrors.WithFields(err, map[string]interface{}{"target": d.u.modName})
	}

	defer r.Close()
	return d.u.moduleError(d.conn.Data(ctx, header, r))
}

func (d *lmtpDelivery) BodyNonAtomic(ctx context.Context, sc module.StatusCollector, header textproto.Header, body buffer.Buffer) {
	r, err := body.Open()
	if err != nil {
		modErr := d.u.moduleError(err)
		for _, rcpt := range d.rcpts {
			sc.SetStatus(rcpt, modErr)
		}
	}
	defer r.Close()

	rcptIndx := 0
	err = d.conn.LMTPData(ctx, header, r, func(rcpt string, err *smtp.SMTPError) {
		if err == nil {
			sc.SetStatus(rcpt, nil)
		} else {
			sc.SetStatus(rcpt, &exterrors.SMTPError{
				Code:         err.Code,
				EnhancedCode: exterrors.EnhancedCode(err.EnhancedCode),
				Message:      err.Message,
				TargetName:   d.u.modName,
				Err:          err,
			})
		}
		rcptIndx++
	})
	if err != nil {
		modErr := d.u.moduleError(err)
		for _, rcpt := range d.rcpts[rcptIndx:] {
			sc.SetStatus(rcpt, modErr)
		}
	}
}

func (d *delivery) Abort(ctx context.Context) error {
	d.conn.Close()
	return nil
}

func (d *delivery) Commit(ctx context.Context) error {
	return d.conn.Close()
}

func init() {
	module.Register("target.smtp", NewDownstream)
	module.Register("target.lmtp", NewDownstream)
}
