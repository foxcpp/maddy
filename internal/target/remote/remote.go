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

// Package remote implements module which does outgoing
// message delivery using servers discovered using DNS MX records.
//
// Implemented interfaces:
// - module.DeliveryTarget
package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	modconfig "github.com/foxcpp/maddy/framework/config/module"
	tls2 "github.com/foxcpp/maddy/framework/config/tls"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/limits"
	"github.com/foxcpp/maddy/internal/smtpconn/pool"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
)

var smtpPort = "25"

func moduleError(err error) error {
	return exterrors.WithFields(err, map[string]interface{}{
		"target": "remote",
	})
}

type Target struct {
	name      string
	hostname  string
	localIP   string
	ipv4      bool
	tlsConfig *tls.Config

	resolver    dns.Resolver
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
	extResolver *dns.ExtResolver

	policies          []module.MXAuthPolicy
	limits            *limits.Group
	allowSecOverride  bool
	relaxedREQUIRETLS bool

	pool           *pool.P
	connReuseLimit int

	Log log.Logger

	connectTimeout    time.Duration
	commandTimeout    time.Duration
	submissionTimeout time.Duration
}

var _ module.DeliveryTarget = &Target{}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("remote: inline arguments are not used")
	}
	// Keep this synchronized with testTarget.
	return &Target{
		name:     instName,
		resolver: dns.DefaultResolver(),
		dialer:   (&net.Dialer{}).DialContext,
		Log:      log.Logger{Name: "remote"},
	}, nil
}

func (rt *Target) Init(cfg *config.Map) error {
	var err error
	rt.extResolver, err = dns.NewExtResolver()
	if err != nil {
		rt.Log.Error("cannot initialize DNSSEC-aware resolver, DNSSEC and DANE are not available", err)
	}

	cfg.String("hostname", true, true, "", &rt.hostname)
	cfg.String("local_ip", false, false, "", &rt.localIP)
	cfg.Bool("force_ipv4", false, false, &rt.ipv4)
	cfg.Bool("debug", true, false, &rt.Log.Debug)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, tls2.TLSClientBlock, &rt.tlsConfig)
	cfg.Custom("mx_auth", false, false, func() (interface{}, error) {
		// Default is "no policies" to follow the principles of explicit
		// configuration (if it is not requested - it is not done).
		return nil, nil
	}, func(cfg *config.Map, n config.Node) (interface{}, error) {
		// Module instance is &PolicyGroup.
		var p *PolicyGroup
		if err := modconfig.GroupFromNode("mx_auth", n.Args, n, cfg.Globals, &p); err != nil {
			return nil, err
		}
		return p.L, nil
	}, &rt.policies)
	cfg.Custom("limits", false, false, func() (interface{}, error) {
		return &limits.Group{}, nil
	}, func(cfg *config.Map, n config.Node) (interface{}, error) {
		var g *limits.Group
		if err := modconfig.GroupFromNode("limits", n.Args, n, cfg.Globals, &g); err != nil {
			return nil, err
		}
		return g, nil
	}, &rt.limits)
	cfg.Bool("requiretls_override", false, true, &rt.allowSecOverride)
	cfg.Bool("relaxed_requiretls", false, true, &rt.relaxedREQUIRETLS)
	cfg.Int("conn_reuse_limit", false, false, 10, &rt.connReuseLimit)
	cfg.Duration("connect_timeout", false, false, 5*time.Minute, &rt.connectTimeout)
	cfg.Duration("command_timeout", false, false, 5*time.Minute, &rt.commandTimeout)
	cfg.Duration("submission_timeout", false, false, 5*time.Minute, &rt.submissionTimeout)

	poolCfg := pool.Config{
		MaxKeys:             20000,
		MaxConnsPerKey:      10,     // basically, max. amount of idle connections in cache
		MaxConnLifetimeSec:  150,    // 2.5 mins, half of recommended idle time from RFC 5321
		StaleKeyLifetimeSec: 60 * 5, // should be bigger than MaxConnLifetimeSec
	}
	cfg.Int("conn_max_idle_count", false, false, 10, &poolCfg.MaxConnsPerKey)
	cfg.Int64("conn_max_idle_time", false, false, 150, &poolCfg.MaxConnLifetimeSec)

	if _, err := cfg.Process(); err != nil {
		return err
	}
	rt.pool = pool.New(poolCfg)

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	rt.hostname, err = idna.ToASCII(rt.hostname)
	if err != nil {
		return fmt.Errorf("remote: cannot represent the hostname as an A-label name: %w", err)
	}

	if rt.localIP != "" {
		addr, err := net.ResolveTCPAddr("tcp", rt.localIP+":0")
		if err != nil {
			return fmt.Errorf("remote: failed to parse local IP: %w", err)
		}
		rt.dialer = (&net.Dialer{
			LocalAddr: addr,
		}).DialContext
	}
	if rt.ipv4 {
		dial := rt.dialer
		rt.dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if network == "tcp" {
				network = "tcp4"
			}
			return dial(ctx, network, addr)
		}
	}

	return nil
}

func (rt *Target) Close() error {
	rt.pool.Close()

	return nil
}

func (rt *Target) Name() string {
	return "remote"
}

func (rt *Target) InstanceName() string {
	return rt.name
}

type remoteDelivery struct {
	rt       *Target
	mailFrom string
	msgMeta  *module.MsgMetadata
	Log      log.Logger

	recipients  []string
	connections map[string]*mxConn

	policies []module.DeliveryMXAuthPolicy
}

func (rt *Target) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	policies := make([]module.DeliveryMXAuthPolicy, 0, len(rt.policies))
	if !(msgMeta.TLSRequireOverride && rt.allowSecOverride) {
		for _, p := range rt.policies {
			policies = append(policies, p.Start(msgMeta))
		}
	}

	var (
		ratelimitDomain string
		err             error
	)
	// This will leave ratelimitDomain = "" for null return path which is fine
	// for purposes of ratelimiting.
	if mailFrom != "" {
		_, ratelimitDomain, err = address.Split(mailFrom)
		if err != nil {
			return nil, &exterrors.SMTPError{
				Code:         501,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 8},
				Message:      "Malformed sender address",
				TargetName:   "remote",
				Err:          err,
			}
		}
	}

	// Domain is already should be normalized by the message source (e.g.
	// endpoint/smtp).
	region := trace.StartRegion(ctx, "remote/limits.Take")
	addr := net.IPv4(127, 0, 0, 1)
	if msgMeta.Conn != nil && msgMeta.Conn.RemoteAddr != nil {
		tcpAddr, ok := msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
		if ok {
			addr = tcpAddr.IP
		}
	}
	if err := rt.limits.TakeMsg(ctx, addr, ratelimitDomain); err != nil {
		region.End()
		return nil, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 4, 5},
			Message:      "High load, try again later",
			Reason:       "Global limit timeout",
			TargetName:   "remote",
			Err:          err,
		}
	}
	region.End()

	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		msgMeta:     msgMeta,
		Log:         target.DeliveryLogger(rt.Log, msgMeta),
		connections: map[string]*mxConn{},
		policies:    policies,
	}, nil
}

func (rd *remoteDelivery) AddRcpt(ctx context.Context, to string) error {
	defer trace.StartRegion(ctx, "remote/AddRcpt").End()

	if rd.msgMeta.Quarantine {
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
			Message:      "Refusing to deliver a quarantined message",
			TargetName:   "remote",
		}
	}

	_, domain, err := address.Split(to)
	if err != nil {
		return err
	}

	// Special-case for <postmaster> address. If it is not handled by a rewrite rule before
	// - we should not attempt to do anything with it and reject it as invalid.
	if domain == "" {
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
			Message:      "<postmaster> address it no supported",
			TargetName:   "remote",
		}
	}

	if strings.HasPrefix(domain, "[") {
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 1},
			Message:      "IP address literals are not supported",
			TargetName:   "remote",
		}
	}

	conn, err := rd.connectionForDomain(ctx, domain)
	if err != nil {
		return err
	}

	if err := conn.Rcpt(ctx, to); err != nil {
		return moduleError(err)
	}

	rd.recipients = append(rd.recipients, to)
	return nil
}

type multipleErrs struct {
	errs      map[string]error
	statusLck sync.Mutex
}

func (m *multipleErrs) Error() string {
	m.statusLck.Lock()
	defer m.statusLck.Unlock()
	return fmt.Sprintf("Partial delivery failure, per-rcpt info: %+v", m.errs)
}

func (m *multipleErrs) Fields() map[string]interface{} {
	m.statusLck.Lock()
	defer m.statusLck.Unlock()

	// If there are any temporary errors - the sender should retry to make sure
	// all recipients will get the message. However, since we can't tell it
	// which recipients got the message, this will generate duplicates for
	// them.
	//
	// We favor delivery with duplicates over incomplete delivery here.

	var (
		code     = 550
		enchCode = exterrors.EnhancedCode{5, 0, 0}
	)
	for _, err := range m.errs {
		if exterrors.IsTemporary(err) {
			code = 451
			enchCode = exterrors.EnhancedCode{4, 0, 0}
		}
	}

	return map[string]interface{}{
		"smtp_code":     code,
		"smtp_enchcode": enchCode,
		"smtp_msg":      "Partial delivery failure, additional attempts may result in duplicates",
		"target":        "remote",
		"errs":          m.errs,
	}
}

func (m *multipleErrs) SetStatus(rcptTo string, err error) {
	m.statusLck.Lock()
	defer m.statusLck.Unlock()
	m.errs[rcptTo] = err
}

func (rd *remoteDelivery) Body(ctx context.Context, header textproto.Header, buffer buffer.Buffer) error {
	defer trace.StartRegion(ctx, "remote/Body").End()

	merr := multipleErrs{
		errs: make(map[string]error),
	}
	rd.BodyNonAtomic(ctx, &merr, header, buffer)

	for _, v := range merr.errs {
		if v != nil {
			if len(merr.errs) == 1 {
				return v
			}
			return &merr
		}
	}
	return nil
}

func (rd *remoteDelivery) BodyNonAtomic(ctx context.Context, c module.StatusCollector, header textproto.Header, b buffer.Buffer) {
	defer trace.StartRegion(ctx, "remote/BodyNonAtomic").End()

	if rd.msgMeta.Quarantine {
		for _, rcpt := range rd.recipients {
			c.SetStatus(rcpt, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Refusing to deliver quarantined message",
				TargetName:   "remote",
			})
		}
		return
	}

	var wg sync.WaitGroup

	for i, conn := range rd.connections {
		i := i
		conn := conn
		wg.Add(1)
		go func() {
			defer wg.Done()

			bodyR, err := b.Open()
			if err != nil {
				for _, rcpt := range conn.Rcpts() {
					c.SetStatus(rcpt, err)
				}
				return
			}
			defer bodyR.Close()

			err = conn.Data(ctx, header, bodyR)
			for _, rcpt := range conn.Rcpts() {
				c.SetStatus(rcpt, err)
			}
			rd.connections[i].errored = err != nil
		}()
	}

	wg.Wait()
}

func (rd *remoteDelivery) Abort(ctx context.Context) error {
	return rd.Close()
}

func (rd *remoteDelivery) Commit(ctx context.Context) error {
	// It is not possible to implement it atomically, so users of remoteDelivery have to
	// take care of partial failures.
	return rd.Close()
}

func (rd *remoteDelivery) Close() error {
	for _, conn := range rd.connections {
		rd.rt.limits.ReleaseDest(conn.domain)
		conn.transactions++

		if conn.C == nil || conn.transactions > rd.rt.connReuseLimit || conn.C.Client() == nil || conn.errored {
			rd.Log.Debugf("disconnected from %s (errored=%v,transactions=%v,disconnected before=%v)",
				conn.ServerName(), conn.errored, conn.transactions, conn.C.Client() == nil)
			conn.Close()
		} else {
			rd.Log.Debugf("returning connection for %s to pool", conn.ServerName())
			rd.rt.pool.Return(conn.domain, conn)
		}
	}

	var (
		ratelimitDomain string
		err             error
	)
	if rd.mailFrom != "" {
		_, ratelimitDomain, err = address.Split(rd.mailFrom)
		if err != nil {
			return err
		}
	}

	addr := net.IPv4(127, 0, 0, 1)
	if rd.msgMeta.Conn != nil && rd.msgMeta.Conn.RemoteAddr != nil {
		tcpAddr, ok := rd.msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
		if ok {
			addr = tcpAddr.IP
		}
	}
	rd.rt.limits.ReleaseMsg(addr, ratelimitDomain)

	return nil
}

func init() {
	module.Register("target.remote", New)
}
