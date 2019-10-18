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
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/dns"
	"github.com/foxcpp/maddy/exterrors"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/mtasts"
	"github.com/foxcpp/maddy/target"
)

const (
	AuthDisabled = "off"
	AuthMTASTS   = "mtasts"
	AuthDNSSEC   = "dnssec"
)

type Target struct {
	name          string
	hostname      string
	requireTLS    bool
	tlsConfig     *tls.Config
	requireMXAuth bool
	mxAuth        map[string]struct{}

	resolver    dns.Resolver
	extResolver *dns.ExtResolver

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
		resolver:    net.DefaultResolver,
		mtastsCache: mtasts.Cache{Resolver: net.DefaultResolver},
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
	cfg.EnumList("authenticate_mx", false, false, []string{AuthDisabled, AuthDNSSEC, AuthMTASTS}, []string{AuthMTASTS}, &mxAuth)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, config.TLSClientBlock, &rt.tlsConfig)
	if _, err := cfg.Process(); err != nil {
		return err
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
		rt.mtastsCache.Logger = &rt.Log
		// MTA-STS policies typically have max_age around one day, so updating them
		// twice a day should keep them up-to-date most of the time.
		rt.stsCacheUpdateTick = time.NewTicker(12 * time.Hour)
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

type remoteConnection struct {
	recipients []string
	serverName string
	*smtp.Client
}

type remoteDelivery struct {
	rt       *Target
	mailFrom string
	msgMeta  *module.MsgMetadata
	Log      log.Logger

	recipients  []string
	connections map[string]*remoteConnection
}

func (rt *Target) Start(msgMeta *module.MsgMetadata, mailFrom string) (module.Delivery, error) {
	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		msgMeta:     msgMeta,
		Log:         target.DeliveryLogger(rt.Log, msgMeta),
		connections: map[string]*remoteConnection{},
	}, nil
}

func (rd *remoteDelivery) AddRcpt(to string) error {
	if rd.msgMeta.Quarantine {
		return errors.New("remote: refusing to deliver quarantined message")
	}

	_, domain, err := address.Split(to)
	if err != nil {
		return err
	}

	// Special-case for <postmaster> address. If it is not handled by a rewrite rule before
	// - we should not attempt to do anything with it and reject it as invalid.
	if domain == "" {
		return fmt.Errorf("<postmaster> address is not supported")
	}

	// serverName (MX serv. address) is very useful for tracing purposes and should be logged on all related errors.
	conn, err := rd.connectionForDomain(domain)
	if err != nil {
		return err
	}

	if err := conn.Rcpt(to); err != nil {
		rd.Log.Printf("RCPT TO %s failed: %v (server = %s)", to, err, conn.serverName)
		return err
	}

	rd.recipients = append(rd.recipients, to)
	conn.recipients = append(conn.recipients, to)
	return nil
}

type multipleErrs struct {
	errs      map[string]error
	statusLck sync.Mutex
}

func (m *multipleErrs) Error() string {
	m.statusLck.Lock()
	defer m.statusLck.Unlock()
	return fmt.Sprintf("remote: partial delivery failure, per-rcpt info: %+v", m.errs)
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
	if len(merr.errs) != 0 {
		return &merr
	}
	return nil
}

func (rd *remoteDelivery) BodyNonAtomic(c module.StatusCollector, header textproto.Header, b buffer.Buffer) {
	if rd.msgMeta.Quarantine {
		for _, rcpt := range rd.recipients {
			c.SetStatus(rcpt, exterrors.WithTemporary(errors.New("remote: refusing to deliver quarantined message"), false))
		}
		return
	}

	var wg sync.WaitGroup

	for _, conn := range rd.connections {
		conn := conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			setErr := func(err error) {
				for _, rcpt := range conn.recipients {
					c.SetStatus(rcpt, err)
				}
			}

			bodyW, err := conn.Data()
			if err != nil {
				rd.Log.Printf("DATA failed: %v (server = %s)", err, conn.serverName)
				setErr(err)
				return
			}
			bodyR, err := b.Open()
			if err != nil {
				rd.Log.Printf("failed to open body buffer: %v", err)
				setErr(err)
				return
			}
			defer bodyR.Close()
			if err = textproto.WriteHeader(bodyW, header); err != nil {
				rd.Log.Printf("header write failed: %v (server = %s)", err, conn.serverName)
				setErr(err)
				return
			}
			if _, err = io.Copy(bodyW, bodyR); err != nil {
				rd.Log.Printf("body write failed: %v (server = %s)", err, conn.serverName)
				setErr(err)
				return
			}

			if err := bodyW.Close(); err != nil {
				rd.Log.Printf("body write final failed: %v (server = %s)", err, conn.serverName)
				setErr(err)
				return
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
		rd.Log.Debugf("disconnected from %s", conn.serverName)
		conn.Close()
	}
	return nil
}

func (rd *remoteDelivery) connectionForDomain(domain string) (*remoteConnection, error) {
	domain = strings.ToLower(domain)

	if c, ok := rd.connections[domain]; ok {
		return c, nil
	}

	addrs, requireTLS, err := rd.lookupAndFilter(domain)
	if err != nil {
		return nil, err
	}
	requireTLS = requireTLS || rd.rt.requireTLS
	if !requireTLS {
		rd.Log.Printf("TLS is not enforced when delivering to %s", domain)
	}

	var lastErr error
	conn := &remoteConnection{}
	for _, addr := range addrs {
		conn.serverName = addr

		conn.Client, err = rd.rt.connectToServer(addr, requireTLS)
		if err != nil {
			rd.Log.Printf("failed to connect to %s: %v", addr, err)
			lastErr = err
			continue
		}
	}
	if conn.Client == nil {
		rd.Log.Printf("no usable MX servers found for %s", domain)
		return nil, lastErr
	}

	if err := conn.Mail(rd.mailFrom); err != nil {
		rd.Log.Printf("MAIL FROM %s failed: %v (server = %s)", rd.mailFrom, err, conn.serverName)
		return nil, err
	}

	rd.Log.Debugf("connected to %s", conn.serverName)
	rd.connections[domain] = conn

	return conn, nil
}

func (rt *Target) getSTSPolicy(domain string) (*mtasts.Policy, error) {
	stsPolicy, err := rt.mtastsCache.Get(domain)
	if err != nil && err != mtasts.ErrNoPolicy {
		rt.Log.Printf("failed to fetch MTA-STS policy for %s: %v", domain, err)
		return nil, exterrors.WithTemporary(err, true)
	}
	return stsPolicy, nil
}

func (rt *Target) stsCacheUpdater() {
	// Always update cache on start-up since we may have been down for some
	// time.
	rt.Log.Debugln("updating MTA-STS cache...")
	if err := rt.mtastsCache.RefreshCache(); err != nil {
		rt.Log.Printf("MTA-STS cache opdate failed: %v", err)
	}
	rt.Log.Debugln("updating MTA-STS cache... done!")

	for {
		select {
		case <-rt.stsCacheUpdateTick.C:
			rt.Log.Debugln("updating MTA-STS cache...")
			if err := rt.mtastsCache.RefreshCache(); err != nil {
				rt.Log.Printf("MTA-STS cache opdate failed: %v", err)
			}
			rt.Log.Debugln("updating MTA-STS cache... done!")
		case <-rt.stsCacheUpdateDone:
			rt.stsCacheUpdateDone <- struct{}{}
			return
		}
	}
}

func (rt *Target) connectToServer(address string, requireTLS bool) (*smtp.Client, error) {
	cl, err := smtp.Dial(address + ":25")
	if err != nil {
		return nil, err
	}

	if err := cl.Hello(rt.hostname); err != nil {
		return nil, err
	}

	if tlsOk, _ := cl.Extension("STARTTLS"); tlsOk {
		cfg := rt.tlsConfig.Clone()
		cfg.ServerName = address
		if err := cl.StartTLS(cfg); err != nil {
			return nil, err
		}
	} else if requireTLS {
		return nil, exterrors.WithTemporary(errors.New("remote: TLS is required but unsupported by MX"), false)
	}

	return cl, nil
}

func (rd *remoteDelivery) lookupAndFilter(domain string) (candidates []string, requireTLS bool, err error) {
	var policy *mtasts.Policy
	if _, use := rd.rt.mxAuth[AuthMTASTS]; use {
		policy, err = rd.rt.getSTSPolicy(domain)
		if err != nil {
			return nil, false, err
		}
		switch policy.Mode {
		case mtasts.ModeEnforce:
			requireTLS = true
		case mtasts.ModeNone:
			policy = nil
		}
	}

	dnssecOk, mxs, err := rd.lookupMX(domain)
	if err != nil {
		return nil, false, err
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

	candidates = make([]string, 0, len(mxs))

	skippedMXs := false
	for _, mx := range mxs {
		authenticated := false
		// If we have MTA-STS - policy match is enough to mark MX as "safe"
		if policy != nil {
			if policy.Match(mx.Host) {
				// Policy in 'testing' mode is enough to authenticate MX too.
				rd.Log.Debugf("authenticated MX (%s) using MTA-STS", mx.Host)
				authenticated = true
			} else if policy.Mode == mtasts.ModeEnforce {
				// Honor *enforced* policy and skip non-matching MXs even if we
				// don't require authentication.
				rd.Log.Printf("ignoring MX (%s) due to MTA-STS", mx.Host)
				skippedMXs = true
				continue
			}
		}
		// If we have DNSSEC - DNSSEC-signed MX record also qualifies as "safe".
		if dnssecOk {
			rd.Log.Debugf("authenticated MX (%s) using DNSSEC", mx.Host)
			authenticated = true
		}

		if !authenticated {
			if rd.rt.requireMXAuth {
				rd.Log.Printf("ignoring non-authenticated MX (%s)", mx.Host)
				skippedMXs = true
				continue
			}
			if _, disabled := rd.rt.mxAuth[AuthDisabled]; !disabled {
				rd.Log.Printf("adding non-authenticated MX (%s) to candidates", mx.Host)
			}
		}

		candidates = append(candidates, mx.Host)
	}

	if len(candidates) == 0 {
		if skippedMXs {
			return nil, false, errors.New("remote: no usable MXs found (ignoring non-authenticated MXs)")
		}
		return nil, false, errors.New("remote: no usable MXs found")
	}

	return candidates, requireTLS, nil
}

func (rd *remoteDelivery) lookupMX(domain string) (dnssecOk bool, records []*net.MX, err error) {
	if _, use := rd.rt.mxAuth[AuthDNSSEC]; use {
		if rd.rt.extResolver == nil {
			return false, nil, errors.New("remote: can't do DNSSEC verification without security-aware resolver")
		}
		return rd.rt.extResolver.AuthLookupMX(context.Background(), domain)
	}

	records, err = rd.rt.resolver.LookupMX(context.Background(), domain)
	return false, records, err
}

func init() {
	module.Register("remote", New)
}
