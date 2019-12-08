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
	"sort"
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
	"github.com/foxcpp/maddy/internal/smtpconn"
	"github.com/foxcpp/maddy/internal/target"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
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
	dialer      func(network, addr string) (net.Conn, error)
	extResolver *dns.ExtResolver

	// This is the callback that is usually mtastsCache.Get,
	// but replaced by tests to mock mtasts.Cache.
	mtastsGet func(domain string) (*mtasts.Policy, error)

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
		dialer:      net.Dial,
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
	connections map[string]*smtpconn.C
}

func (rt *Target) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		msgMeta:     msgMeta,
		Log:         target.DeliveryLogger(rt.Log, msgMeta),
		connections: map[string]*smtpconn.C{},
	}, nil
}

func (rd *remoteDelivery) AddRcpt(to string) error {
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

	conn, err := rd.connectionForDomain(domain)
	if err != nil {
		return err
	}

	if err := conn.Rcpt(to); err != nil {
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

func (rd *remoteDelivery) Body(header textproto.Header, buffer buffer.Buffer) error {
	merr := multipleErrs{
		errs: make(map[string]error),
	}
	rd.BodyNonAtomic(&merr, header, buffer)

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

func (rd *remoteDelivery) BodyNonAtomic(c module.StatusCollector, header textproto.Header, b buffer.Buffer) {
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

			err = conn.Data(header, bodyR)
			for _, rcpt := range conn.Rcpts() {
				c.SetStatus(rcpt, err)
			}
		}()
	}

	wg.Wait()
}

func (rd *remoteDelivery) Abort() error {
	return rd.Close()
}

func (rd *remoteDelivery) Commit() error {
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

func (rd *remoteDelivery) connectionForDomain(domain string) (*smtpconn.C, error) {
	domain = strings.ToLower(domain)

	if c, ok := rd.connections[domain]; ok {
		return c, nil
	}

	authMXs, nonAuthMXs, requireTLS, err := rd.lookupAndFilter(domain)
	if err != nil {
		return nil, err
	}
	requireTLS = requireTLS || rd.rt.requireTLS
	if !requireTLS {
		rd.Log.Msg("TLS not enforced", "domain", domain)
	}

	var (
		lastErr error
		conn    = smtpconn.New()
	)

	conn.Dialer = rd.rt.dialer
	conn.RequireTLS = rd.rt.requireTLS
	conn.AttemptTLS = true
	conn.TLSConfig = rd.rt.tlsConfig
	conn.Log = rd.Log
	conn.Hostname = rd.rt.hostname
	conn.AddrInSMTPMsg = true

	addrs := append(make([]string, 0, len(authMXs)+len(nonAuthMXs)), authMXs...)
	addrs = append(addrs, nonAuthMXs...)
	rd.Log.DebugMsg("considering", "mxs", addrs)
	for i, addr := range addrs {
		err = conn.Connect(config.Endpoint{
			Scheme: "tcp",
			Host:   addr,
			Port:   smtpPort,
		})
		if err != nil {
			if len(addrs) != 1 {
				rd.Log.Error("connect error", err, "remote_server", addr, "domain", domain)
			}
			lastErr = err
			continue
		}
		if _, disabled := rd.rt.mxAuth[AuthDisabled]; !disabled && i >= len(authMXs) {
			rd.Log.Msg("using non-authenticated MX", "remote_server", addr, "domain", domain)
		}
	}
	if conn.Client() == nil {
		if lastErr == nil {
			return nil, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
				Message:      "No usable MXs",
				TargetName:   "remote",
			}

		}
		return nil, moduleError(lastErr)
	}

	rd.Log.DebugMsg("connected", "remote_server", conn.ServerName())

	if err := conn.Mail(rd.mailFrom, rd.msgMeta.SMTPOpts); err != nil {
		conn.Close()
		return nil, moduleError(err)
	}

	rd.connections[domain] = conn
	return conn, nil
}

func (rt *Target) getSTSPolicy(domain string) (*mtasts.Policy, error) {
	stsPolicy, err := rt.mtastsGet(domain)
	if err != nil && !mtasts.IsNoPolicy(err) {
		return nil, &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(err, 450, 554),
			EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 0, 0}),
			Message:      "Failed to fetch the recipient policy",
			TargetName:   "remote",
			Err:          err,
			Misc: map[string]interface{}{
				"domain": domain,
			},
		}
	}
	return stsPolicy, nil
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

func (rd *remoteDelivery) lookupAndFilter(domain string) (authMXs, nonAuthMXs []string, requireTLS bool, err error) {
	var policy *mtasts.Policy
	if _, use := rd.rt.mxAuth[AuthMTASTS]; use {
		policy, err = rd.rt.getSTSPolicy(domain)
		if err != nil {
			return nil, nil, false, err
		}
		if policy != nil {
			switch policy.Mode {
			case mtasts.ModeEnforce:
				requireTLS = true
			case mtasts.ModeNone:
				policy = nil
			}
		}
	}

	dnssecOk, mxs, err := rd.lookupMX(domain)
	if err != nil {
		return nil, nil, false, err
	}

	// Fallback to A/AAA RR when no MX records are present as
	// required by RFC 5321 Section 5.1.
	if len(mxs) == 0 {
		mxs = append(mxs, &net.MX{
			Host: domain,
			Pref: 0,
		})
	}

	sort.Slice(mxs, func(i, j int) bool {
		return mxs[i].Pref < mxs[j].Pref
	})

	authMXs = make([]string, 0, len(mxs))
	nonAuthMXs = make([]string, 0, len(mxs))

	for _, mx := range mxs {
		if mx.Host == "." {
			return nil, nil, false, &exterrors.SMTPError{
				Code:         556,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 10},
				Message:      "Domain does not accept email (null MX)",
			}
		}

		authenticated := false

		// This is exactly the domain we wanted to want to send a message to.
		if dns.Equal(mx.Host, domain) {
			authenticated = true
		}

		// If we have MTA-STS - policy match is enough to mark MX as "safe"
		if policy != nil {
			if policy.Match(mx.Host) {
				// Policy in 'testing' mode is enough to authenticate MX too.
				rd.Log.DebugMsg("authenticated MX using MTA-STS", "mx", mx.Host, "domain", domain)
				authenticated = true
			} else if policy.Mode == mtasts.ModeEnforce {
				// Honor *enforced* policy and skip non-matching MXs even if we
				// don't require authentication.
				rd.Log.Msg("ignoring MX due to MTA-STS", "mx", mx.Host, "domain", domain)
				continue
			}
		}
		// If we have DNSSEC - DNSSEC-signed MX record also qualifies as "safe".
		if dnssecOk {
			rd.Log.DebugMsg("authenticated MX using DNSSEC", "mx", mx.Host, "domain", domain)
			authenticated = true
		}
		if _, use := rd.rt.mxAuth[AuthCommonDomain]; use && commonDomainCheck(domain, mx.Host) {
			rd.Log.DebugMsg("authenticated MX using common domain rule", "mx", mx.Host, "domain", domain)
			authenticated = true
		}

		if !authenticated {
			if rd.rt.requireMXAuth {
				rd.Log.Msg("ignoring non-authenticated MX", "mx", mx.Host, "domain", domain)
				continue
			}
		}

		if authenticated {
			authMXs = append(authMXs, mx.Host)
		} else {
			nonAuthMXs = append(nonAuthMXs, mx.Host)
		}
	}

	return authMXs, nonAuthMXs, requireTLS, nil
}

func commonDomainCheck(domain string, mx string) bool {
	domainPart, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return false
	}
	mxPart, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimSuffix(mx, "."))
	if err != nil {
		return false
	}

	return domainPart == mxPart
}

func (rd *remoteDelivery) lookupMX(domain string) (dnssecOk bool, records []*net.MX, err error) {
	if _, use := rd.rt.mxAuth[AuthDNSSEC]; use {
		if rd.rt.extResolver == nil {
			return false, nil, errors.New("remote: can't do DNSSEC verification without security-aware resolver")
		}
		ad, records, err := rd.rt.extResolver.AuthLookupMX(context.Background(), domain)
		if err != nil {
			code := 554
			enchCode := exterrors.EnhancedCode{5, 4, 4}
			if exterrors.IsTemporary(err) {
				code = 451
				enchCode = exterrors.EnhancedCode{4, 4, 4}
			}
			return false, nil, &exterrors.SMTPError{
				Code:         code,
				EnhancedCode: enchCode,
				Message:      "MX lookup error",
				TargetName:   "remote",
				Err:          err,
			}
		}
		return ad, records, nil
	}

	records, err = rd.rt.resolver.LookupMX(context.Background(), domain)
	if err != nil {
		code := 554
		enchCode := exterrors.EnhancedCode{5, 4, 4}
		if exterrors.IsTemporary(err) {
			code = 451
			enchCode = exterrors.EnhancedCode{4, 4, 4}
		}
		return false, nil, &exterrors.SMTPError{
			Code:         code,
			EnhancedCode: enchCode,
			Message:      "MX lookup error",
			TargetName:   "remote",
			Err:          err,
		}
	}
	return false, records, err
}

func init() {
	module.Register("remote", New)
}
