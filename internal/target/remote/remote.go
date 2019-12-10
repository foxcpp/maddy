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
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/internal/address"
	"github.com/foxcpp/maddy/internal/buffer"
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/mtasts"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
)

const (
	AuthDisabled     = "off"
	AuthMTASTS       = "mtasts"
	AuthDNSSEC       = "dnssec"
	AuthCommonDomain = "common_domain"
)

var smtpPort = "25"

func moduleError(err error) error {
	return exterrors.WithFields(err, map[string]interface{}{
		"target": "remote",
	})
}

type Target struct {
	name          string
	hostname      string
	requireTLS    bool
	tlsConfig     *tls.Config
	requireMXAuth bool
	mxAuth        map[string]struct{}

	resolver    dns.Resolver
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
	extResolver *dns.ExtResolver

	// This is the callback that is usually mtastsCache.Get,
	// but replaced by tests to mock mtasts.Cache.
	mtastsGet func(ctx context.Context, domain string) (*mtasts.Policy, error)

	mtastsCache        mtasts.Cache
	stsCacheUpdateTick *time.Ticker
	stsCacheUpdateDone chan struct{}

	Log log.Logger
}

var _ module.DeliveryTarget = &Target{}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	if len(inlineArgs) != 0 {
		return nil, errors.New("remote: inline arguments are not used")
	}
	return &Target{
		name:        instName,
		resolver:    dns.DefaultResolver(),
		dialer:      (&net.Dialer{}).DialContext,
		mtastsCache: mtasts.Cache{Resolver: dns.DefaultResolver()},
		Log:         log.Logger{Name: "remote"},

		stsCacheUpdateDone: make(chan struct{}),
	}, nil
}

func (rt *Target) Init(cfg *config.Map) error {
	var mxAuth []string

	cfg.String("hostname", true, true, "", &rt.hostname)
	cfg.String("mtasts_cache", false, false, filepath.Join(config.StateDirectory, "mtasts-cache"), &rt.mtastsCache.Location)
	cfg.Bool("debug", true, false, &rt.Log.Debug)
	cfg.Bool("require_tls", false, false, &rt.requireTLS)
	cfg.EnumList("authenticate_mx", false, false,
		[]string{AuthDisabled, AuthDNSSEC, AuthMTASTS, AuthCommonDomain},
		[]string{AuthMTASTS, AuthDNSSEC}, &mxAuth)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, config.TLSClientBlock, &rt.tlsConfig)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	var err error
	rt.hostname, err = idna.ToASCII(rt.hostname)
	if err != nil {
		return fmt.Errorf("remote: cannot represent the hostname as an A-label name: %w", err)
	}

	if err := rt.initMXAuth(mxAuth); err != nil {
		return err
	}

	return nil
}

func (rt *Target) initMXAuth(methods []string) error {
	rt.mxAuth = make(map[string]struct{})
	for _, method := range methods {
		rt.mxAuth[method] = struct{}{}
	}

	if _, disabled := rt.mxAuth[AuthDisabled]; disabled && len(rt.mxAuth) > 1 {
		return errors.New("remote: can't use 'off' together with other options in authenticate_mx")
	}

	if _, use := rt.mxAuth[AuthDNSSEC]; use {
		var err error
		rt.extResolver, err = dns.NewExtResolver()
		if err != nil {
			return fmt.Errorf("remote: failed to init DNSSEC-aware stub resolver: %w", err)
		}
	}

	// Without MX authentication, TLS is fragile.
	if _, disabled := rt.mxAuth[AuthDisabled]; !disabled && rt.requireTLS {
		rt.Log.Printf("MX authentication is enforced due to require_tls")
		rt.requireMXAuth = true
	}

	if _, use := rt.mxAuth[AuthMTASTS]; use {
		if err := os.MkdirAll(rt.mtastsCache.Location, os.ModePerm); err != nil {
			return err
		}
		rt.mtastsCache.Logger = log.Logger{Name: "remote/mtasts", Debug: rt.Log.Debug}
		// MTA-STS policies typically have max_age around one day, so updating them
		// twice a day should keep them up-to-date most of the time.
		rt.stsCacheUpdateTick = time.NewTicker(12 * time.Hour)
		rt.mtastsGet = rt.mtastsCache.Get
		go rt.stsCacheUpdater()
	}

	return nil
}

func (rt *Target) Close() error {
	if _, use := rt.mxAuth[AuthMTASTS]; use {
		rt.stsCacheUpdateDone <- struct{}{}
		<-rt.stsCacheUpdateDone
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
}

func (rt *Target) Start(ctx context.Context, msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		msgMeta:     msgMeta,
		Log:         target.DeliveryLogger(rt.Log, msgMeta),
		connections: map[string]mxConn{},
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
			Message:      "<postmaster> address it no ssupported",
			TargetName:   "remote",
		}
	}

	if strings.HasPrefix(domain, "[") {
		domain, err = rd.domainFromIP(domain)
		if err != nil {
			return err
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

func (rd *remoteDelivery) domainFromIP(ipLiteral string) (string, error) {
	if !strings.HasSuffix(ipLiteral, "]") {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Malformed address literal",
			TargetName:   "remote",
		}
	}
	ip := ipLiteral[1 : len(ipLiteral)-1]
	ip = strings.TrimPrefix(ip, "IPv6:")

	if !rd.rt.requireMXAuth {
		names, err := rd.rt.resolver.LookupAddr(context.Background(), ip)
		if err != nil {
			return "", &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
				Message:      "Cannot use that recipient IP",
				TargetName:   "remote",
				Err:          err,
			}
		}
		if len(names) == 0 {
			return "", &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
				Message:      "Cannot use this recipient IP",
				TargetName:   "remote",
				Reason:       "Missing the PTR record",
			}
		}

		return names[0], nil
	}

	// With required MX authentication, we require DNSSEC-signed PTR.
	// Most ISPs don't provide this, so this will fail 99.99% of times.

	if _, ok := rd.rt.mxAuth[AuthDNSSEC]; !ok {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Cannot authenticate the recipient IP",
			TargetName:   "remote",
			Reason:       "DNSSEC authentication should be enabled for IP literals to be accepted",
		}
	}

	if rd.rt.extResolver == nil {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Cannot authenticate the recipient IP",
			TargetName:   "remote",
			Reason:       "Cannot do DNSSEC authentication without a security-aware resolver",
		}
	}

	ad, names, err := rd.rt.extResolver.AuthLookupAddr(context.Background(), ip)
	if err != nil {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Cannot use that recipient IP",
			TargetName:   "remote",
			Err:          err,
		}
	}
	if len(names) == 0 {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Used recipient IP lacks a rDNS name",
			TargetName:   "remote",
		}
	}
	if !ad {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Cannot authenticate the recipient IP",
			TargetName:   "remote",
			Reason:       "Reverse zone is not DNSSEC-signed",
		}
	}

	return names[0], nil
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

func (rt *Target) stsCacheUpdater() {
	// Always update cache on start-up since we may have been down for some
	// time.
	rt.Log.Debugln("updating MTA-STS cache...")
	if err := rt.mtastsCache.Refresh(); err != nil {
		rt.Log.Msg("MTA-STS cache update error", err)
	}
	rt.Log.Debugln("updating MTA-STS cache... done!")

	for {
		select {
		case <-rt.stsCacheUpdateTick.C:
			rt.Log.Debugln("updating MTA-STS cache...")
			if err := rt.mtastsCache.Refresh(); err != nil {
				rt.Log.Msg("MTA-STS cache opdate error", err)
			}
			rt.Log.Debugln("updating MTA-STS cache... done!")
		case <-rt.stsCacheUpdateDone:
			rt.stsCacheUpdateDone <- struct{}{}
			return
		}
	}
}

func init() {
	module.Register("remote", New)
}
