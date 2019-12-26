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

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
)

const (
	AuthDisabled     = "off"
	AuthMTASTS       = "mtasts"
	AuthDNSSEC       = "dnssec"
	AuthCommonDomain = "common_domain"
)

type (
	TLSLevel int
	MXLevel  int
)

const (
	TLSNone TLSLevel = iota
	TLSEncrypted
	TLSAuthenticated

	MXNone MXLevel = iota
	MX_MTASTS
	MX_DNSSEC
)

func (l TLSLevel) String() string {
	switch l {
	case TLSNone:
		return "none"
	case TLSEncrypted:
		return "encrypted"
	case TLSAuthenticated:
		return "authenticated"
	}
	return "???"
}

func (l MXLevel) String() string {
	switch l {
	case MXNone:
		return "none"
	case MX_MTASTS:
		return "mtasts"
	case MX_DNSSEC:
		return "dnssec"
	}
	return "???"
}

var smtpPort = "25"

func moduleError(err error) error {
	return exterrors.WithFields(err, map[string]interface{}{
		"target": "remote",
	})
}

type Target struct {
	name      string
	hostname  string
	tlsConfig *tls.Config

	resolver    dns.Resolver
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
	extResolver *dns.ExtResolver

	policies    []Policy
	localPolicy *localPolicy

	Log log.Logger
}

var _ module.DeliveryTarget = &Target{}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("remote: inline arguments are not used")
	}
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
	cfg.Bool("debug", true, false, &rt.Log.Debug)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, config.TLSClientBlock, &rt.tlsConfig)

	policies := make([]Policy, 4)
	cfg.Custom("mtasts", false, false,
		rt.defaultPolicy(cfg.Globals, "mtasts"), rt.policyMatcher("mtasts"), &policies[0])
	// sts_preload should go after mtasts so it will take not effect if MXLevel is already MX_MTASTS.
	cfg.Custom("sts_preload", false, false,
		rt.defaultPolicy(cfg.Globals, "sts_preload"), rt.policyMatcher("sts_preload"), &policies[1])
	cfg.Custom("dane", false, false,
		rt.defaultPolicy(cfg.Globals, "dane"), rt.policyMatcher("dane"), &policies[2])
	cfg.Custom("dnssec", false, false,
		rt.defaultPolicy(cfg.Globals, "dnssec"), rt.policyMatcher("dnssec"), &policies[3])

	var localPolicy localPolicy

	// localPolicy should be the last one, since it considers levels defined by
	// other policies.
	// Also, it should be directly accessible from tests to adjust required levels.
	cfg.Custom("local_policy", false, false,
		rt.defaultPolicy(cfg.Globals, "local"), rt.policyMatcher("local"), &localPolicy)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	// Exclude disabled policies.
	for _, p := range policies {
		if p == nil {
			continue
		}
		rt.policies = append(rt.policies, p)
	}
	rt.localPolicy = &localPolicy
	rt.policies = append(rt.policies, &localPolicy)

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	rt.hostname, err = idna.ToASCII(rt.hostname)
	if err != nil {
		return fmt.Errorf("remote: cannot represent the hostname as an A-label name: %w", err)
	}

	return nil
}

func (rt *Target) Close() error {
	for _, p := range rt.policies {
		p.Close()
	}

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
	connections map[string]mxConn

	policies []DeliveryPolicy
}

func (rt *Target) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	// TODO: Consider using sync.Pool?
	policies := make([]DeliveryPolicy, 0, len(rt.policies))
	for _, p := range rt.policies {
		policies = append(policies, p.Start(msgMeta))
	}

	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		msgMeta:     msgMeta,
		Log:         target.DeliveryLogger(rt.Log, msgMeta),
		connections: map[string]mxConn{},
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

	for _, conn := range rd.connections {
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
		rd.Log.Debugf("disconnected from %s", conn.ServerName())
		conn.Close()
	}
	return nil
}

func init() {
	module.Register("remote", New)
}
