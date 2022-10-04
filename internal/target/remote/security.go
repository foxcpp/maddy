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

package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"runtime/debug"
	"time"

	"github.com/foxcpp/go-mtasts"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/future"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/target"
)

type (
	mtastsPolicy struct {
		cache       *mtasts.Cache
		mtastsGet   func(context.Context, string) (*mtasts.Policy, error)
		updaterStop chan struct{}
		log         log.Logger
		instName    string
	}
	mtastsDelivery struct {
		c         *mtastsPolicy
		domain    string
		policyFut *future.Future
		log       log.Logger
	}
)

func NewMTASTSPolicy(_, instName string, _, _ []string) (module.Module, error) {
	return &mtastsPolicy{
		instName: instName,
		log:      log.Logger{Name: "mx_auth.mtasts", Debug: log.DefaultLogger.Debug},
	}, nil
}

func (c *mtastsPolicy) Name() string {
	return c.log.Name
}

func (c *mtastsPolicy) InstanceName() string {
	return c.instName
}

func (c *mtastsPolicy) Weight() int {
	return 10
}

func (c *mtastsPolicy) Init(cfg *config.Map) error {
	var (
		storeType string
		storeDir  string
	)
	cfg.Enum("cache", false, false, []string{"ram", "fs"}, "fs", &storeType)
	cfg.String("fs_dir", false, false, "mtasts_cache", &storeDir)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	switch storeType {
	case "fs":
		if err := os.MkdirAll(storeDir, os.ModePerm); err != nil {
			return err
		}
		c.cache = mtasts.NewFSCache(storeDir)
	case "ram":
		c.cache = mtasts.NewRAMCache()
	default:
		panic("mtasts policy init: unknown cache type")
	}
	c.cache.Resolver = dns.DefaultResolver()
	c.mtastsGet = c.cache.Get

	return nil
}

// StartUpdater starts a goroutine to update MTA-STS cache periodically until
// Close is called.
//
// It can be called only once per mtastsPolicy instance.
func (c *mtastsPolicy) StartUpdater() {
	c.updaterStop = make(chan struct{})
	go c.updater()
}

func (c *mtastsPolicy) updater() {
	defer func() {
		if err := recover(); err != nil {
			stack := debug.Stack()
			log.Printf("panic during MTA-STS update: %v\n%s", err, stack)
			log.Printf("MTA-STS cache refresh disabled due to critical error")
			c.updaterStop = nil
		}
	}()

	// Always update cache on start-up since we may have been down for some
	// time.
	c.log.Debugln("updating MTA-STS cache...")
	if err := c.cache.Refresh(); err != nil {
		c.log.Error("MTA-STS cache update error", err)
	}
	c.log.Debugln("updating MTA-STS cache... done!")

	t := time.NewTicker(12 * time.Hour)
	for {
		select {
		case <-t.C:
			c.log.Debugln("updating MTA-STS cache...")
			if err := c.cache.Refresh(); err != nil {
				c.log.Error("MTA-STS cache opdate error", err)
			}
			c.log.Debugln("updating MTA-STS cache... done!")
		case <-c.updaterStop:
			c.updaterStop <- struct{}{}
			return
		}
	}
}

func (c *mtastsPolicy) Start(msgMeta *module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return &mtastsDelivery{
		c:   c,
		log: target.DeliveryLogger(c.log, msgMeta),
	}
}

func (c *mtastsPolicy) Close() error {
	if c.updaterStop != nil {
		c.updaterStop <- struct{}{}
		<-c.updaterStop
		c.updaterStop = nil
	}
	return nil
}

func (c *mtastsDelivery) PrepareDomain(ctx context.Context, domain string) {
	c.policyFut = future.New()
	go func() {
		c.policyFut.Set(c.c.mtastsGet(ctx, domain))
	}()
}

func (c *mtastsDelivery) PrepareConn(ctx context.Context, mx string) {}

func (c *mtastsDelivery) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	policyI, err := c.policyFut.GetContext(ctx)
	if err != nil {
		c.log.DebugMsg("MTA-STS error", "err", err)
		return module.MXNone, nil
	}
	policy := policyI.(*mtasts.Policy)

	if !policy.Match(mx) {
		if policy.Mode == mtasts.ModeEnforce {
			return module.MXNone, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Failed to establish the module.MX record authenticity (MTA-STS)",
			}
		}
		c.log.Msg("MX does not match published non-enforced MTA-STS policy", "mx", mx, "domain", c.domain)
		return module.MXNone, nil
	}
	return module.MX_MTASTS, nil
}

func (c *mtastsDelivery) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	policyI, err := c.policyFut.GetContext(ctx)
	if err != nil {
		c.c.log.DebugMsg("MTA-STS error", "err", err)
		return module.TLSNone, nil
	}
	policy := policyI.(*mtasts.Policy)

	if policy.Mode != mtasts.ModeEnforce {
		return module.TLSNone, nil
	}

	if !tlsState.HandshakeComplete {
		return module.TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS is required but unavailable or failed (MTA-STS)",
		}
	}

	if tlsState.VerifiedChains == nil {
		return module.TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message: "Recipient server module.TLS certificate is not trusted but " +
				"authentication is required by MTA-STS",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}

	return module.TLSNone, nil
}

func (c *mtastsDelivery) Reset(msgMeta *module.MsgMetadata) {
	c.policyFut = nil
	if msgMeta != nil {
		c.log = target.DeliveryLogger(c.c.log, msgMeta)
	}
}

// Stub that will be removed in 0.5.
type stsPreloadPolicy struct {
	log      log.Logger
	instName string
}

func NewSTSPreload(_, instName string, _, _ []string) (module.Module, error) {
	return &stsPreloadPolicy{
		instName: instName,
		log:      log.Logger{Name: "mx_auth.sts_preload", Debug: log.DefaultLogger.Debug},
	}, nil
}

func (c *stsPreloadPolicy) Name() string {
	return c.log.Name
}

func (c *stsPreloadPolicy) InstanceName() string {
	return c.instName
}

func (c *stsPreloadPolicy) Weight() int {
	return 30 // after MTA-STS
}

func (c *stsPreloadPolicy) Init(cfg *config.Map) error {
	c.log.Println("sts_preload module is deprecated and is no-op as the list is expired and unmaintained")

	var (
		sourcePath     string
		enforceTesting bool
	)
	cfg.String("source", false, false, "eff", &sourcePath)
	cfg.Bool("enforce_testing", false, true, &enforceTesting)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	return nil
}

type preloadDelivery struct {
	*stsPreloadPolicy
}

func (p *stsPreloadPolicy) Start(*module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return &preloadDelivery{stsPreloadPolicy: p}
}

func (p *preloadDelivery) Reset(*module.MsgMetadata)                        {}
func (p *preloadDelivery) PrepareDomain(ctx context.Context, domain string) {}
func (p *preloadDelivery) PrepareConn(ctx context.Context, mx string)       {}
func (p *preloadDelivery) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	return mxLevel, nil
}

func (p *preloadDelivery) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	return tlsLevel, nil
}

func (p *stsPreloadPolicy) Close() error {
	return nil
}

type dnssecPolicy struct {
	instName string
}

func NewDNSSECPolicy(_, instName string, _, _ []string) (module.Module, error) {
	return &dnssecPolicy{
		instName: instName,
	}, nil
}

func (c *dnssecPolicy) Name() string {
	return "mx_auth.dnssec"
}

func (c *dnssecPolicy) InstanceName() string {
	return c.instName
}

func (c *dnssecPolicy) Weight() int {
	return 1
}

func (c *dnssecPolicy) Init(cfg *config.Map) error {
	_, err := cfg.Process() // will fail if there is any directive
	return err
}

func (dnssecPolicy) Start(*module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return dnssecPolicy{}
}

func (dnssecPolicy) Close() error {
	return nil
}

func (dnssecPolicy) Reset(*module.MsgMetadata)                        {}
func (dnssecPolicy) PrepareDomain(ctx context.Context, domain string) {}
func (dnssecPolicy) PrepareConn(ctx context.Context, mx string)       {}

func (dnssecPolicy) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	if dnssec {
		return module.MX_DNSSEC, nil
	}
	return module.MXNone, nil
}

func (dnssecPolicy) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	return module.TLSNone, nil
}

type (
	danePolicy struct {
		extResolver *dns.ExtResolver
		log         log.Logger
		instName    string
	}
	daneDelivery struct {
		c       *danePolicy
		tlsaFut *future.Future
	}
)

func NewDANEPolicy(_, instName string, _, _ []string) (module.Module, error) {
	return &danePolicy{
		instName: instName,
		log:      log.Logger{Name: "remote/dane", Debug: log.DefaultLogger.Debug},
	}, nil
}

func (c *danePolicy) Name() string {
	return "mx_auth.dane"
}

func (c *danePolicy) InstanceName() string {
	return c.instName
}

func (c *danePolicy) Weight() int {
	return 10
}

func (c *danePolicy) Init(cfg *config.Map) error {
	var err error
	c.extResolver, err = dns.NewExtResolver()
	if err != nil {
		c.log.Error("DANE support is no-op: unable to init EDNS resolver", err)
	}

	cfg.Bool("debug", true, log.DefaultLogger.Debug, &c.log.Debug)

	_, err = cfg.Process()
	return err
}

func (c *danePolicy) Start(*module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return &daneDelivery{c: c}
}

func (c *danePolicy) Close() error {
	return nil
}

func (c *daneDelivery) PrepareDomain(ctx context.Context, domain string) {}

func (c *daneDelivery) discoverTLSA(ctx context.Context, mx string) ([]dns.TLSA, error) {
	adA, rname, err := c.c.extResolver.CheckCNAMEAD(ctx, mx)
	if err != nil {
		// This may indicate a bogus DNSSEC signature or other lookup issue
		// (including non-existing domain).
		// Per RFC 7672, any I/O errors (including SERVFAIL) should
		// cause delivery to be delayed.
		return nil, err
	}
	if rname == "" {
		// No A/AAAA records, short-circuit discovery instead of doing useless
		// queries.
		return nil, errors.New("no address associated with the host")
	}
	if !adA {
		// If A lookup is not DNSSEC-authenticated we assume the server cannot
		// have TLSA record and skip trying to actually lookup TLSA
		// to avoid hitting weird errors like SERVFAIL, NOTIMP
		// e.g. see https://github.com/foxcpp/maddy/issues/287
		if rname == mx {
			c.c.log.Debugln("skipping DANE for", mx, "due to non-authenticated A records")
			return nil, nil
		}

		// But if it is CNAME'd then we may not want to skip it and actually
		// consider initial name since it may be signed. To confirm the
		// initial name is signed, do CNAME lookup.
		cnameAD, _, err := c.c.extResolver.AuthLookupCNAME(ctx, mx)
		if err != nil {
			return nil, err
		}
		if !cnameAD {
			c.c.log.Debugln("skipping DANE for", mx, "due to non-authenticated CNAME record")
			return nil, nil
		}
	}

	// If there was a CNAME - try it first.
	if rname != mx {
		ad, recs, err := c.c.extResolver.AuthLookupTLSA(ctx, "25", "tcp", rname)
		if err != nil && !dns.IsNotFound(err) {
			return nil, err
		}
		if ad && len(recs) != 0 {
			// recs may be empty or contain only unusable records - this is
			// okay per RFC 7672, no fallback to initial name is done.
			c.c.log.Debugln("using", len(recs), "DANE records at", rname, "to authenticate", mx)
			return recs, nil
		}
		// Per RFC 7672 Section 2.2 we interpret a non-authenticated RRset just
		// like an empty RRset and fallback to trying original name.
		c.c.log.Debugln("ignoring non-authenticated TLSA records for", rname)
	}

	// If initial name is not a CNAME or final canonical name is not "secure"
	// - we consider TLSA under the initial name.
	ad, recs, err := c.c.extResolver.AuthLookupTLSA(ctx, "25", "tcp", mx)
	if err != nil && !dns.IsNotFound(err) {
		return nil, err
	}
	if !ad {
		c.c.log.Debugln("ignoring non-authenticated TLSA records for", mx)
		return nil, nil
	}

	c.c.log.Debugln("using", len(recs), "DANE records at original name to authenticate", mx)
	return recs, nil
}

func (c *daneDelivery) PrepareConn(ctx context.Context, mx string) {
	// No DNSSEC support.
	if c.c.extResolver == nil {
		return
	}

	c.tlsaFut = future.New()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("panic during extended resolver lookup: %v\n%s", err, stack)
			}
		}()

		c.tlsaFut.Set(c.discoverTLSA(ctx, dns.FQDN(mx)))
	}()
}

func (c *daneDelivery) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	return module.MXNone, nil
}

func (c *daneDelivery) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	// No DNSSEC support.
	if c.c.extResolver == nil {
		return module.TLSNone, nil
	}

	recsI, err := c.tlsaFut.GetContext(ctx)
	if err != nil {
		// No records.
		if dns.IsNotFound(err) {
			return module.TLSNone, nil
		}

		// Lookup error here indicates a resolution failure or may also
		// indicate a bogus DNSSEC signature.
		// There is a big problem with differentiating these two.
		//
		// We assume DANE failure in both cases as a safety measure.
		// However, there is a possibility of a temporary error condition,
		// so we mark it as such.
		return module.TLSNone, exterrors.WithTemporary(err, true)
	}
	recs := recsI.([]dns.TLSA)

	overridePKIX, err := verifyDANE(recs, tlsState)
	if err != nil {
		return module.TLSNone, err
	}
	if overridePKIX {
		return module.TLSAuthenticated, nil
	}
	return module.TLSNone, nil
}

func (c *daneDelivery) Reset(*module.MsgMetadata) {}

type (
	localPolicy struct {
		instName    string
		minTLSLevel module.TLSLevel
		minMXLevel  module.MXLevel
	}
)

func NewLocalPolicy(_, instName string, _, _ []string) (module.Module, error) {
	return &localPolicy{
		instName: instName,
	}, nil
}

func (c *localPolicy) Name() string {
	return "mx_auth.local_policy"
}

func (c *localPolicy) InstanceName() string {
	return c.instName
}

func (c *localPolicy) Weight() int {
	return 1000
}

func (c *localPolicy) Init(cfg *config.Map) error {
	var (
		minTLSLevel string
		minMXLevel  string
	)

	cfg.Enum("min_tls_level", false, false,
		[]string{"none", "encrypted", "authenticated"}, "encrypted", &minTLSLevel)
	cfg.Enum("min_mx_level", false, false,
		[]string{"none", "mtasts", "dnssec"}, "none", &minMXLevel)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	// Enum checks the value against allowed list, no 'default' necessary.
	switch minTLSLevel {
	case "none":
		c.minTLSLevel = module.TLSNone
	case "encrypted":
		c.minTLSLevel = module.TLSEncrypted
	case "authenticated":
		c.minTLSLevel = module.TLSAuthenticated
	}
	switch minMXLevel {
	case "none":
		c.minMXLevel = module.MXNone
	case "mtasts":
		c.minMXLevel = module.MX_MTASTS
	case "dnssec":
		c.minMXLevel = module.MX_DNSSEC
	}

	return nil
}

func (l localPolicy) Start(msgMeta *module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return l
}

func (l localPolicy) Close() error {
	return nil
}

func (l localPolicy) Reset(*module.MsgMetadata)                        {}
func (l localPolicy) PrepareDomain(ctx context.Context, domain string) {}
func (l localPolicy) PrepareConn(ctx context.Context, mx string)       {}

func (l localPolicy) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	if mxLevel < l.minMXLevel {
		return module.MXNone, &exterrors.SMTPError{
			// Err on the side of caution if policy evaluation was messed up by
			// a temporary error (we can't know with the current design).
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
			Message:      "Failed to establish the module.MX record authenticity",
			Misc: map[string]interface{}{
				"mx_level": mxLevel,
			},
		}
	}
	return module.MXNone, nil
}

func (l localPolicy) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	if tlsLevel < l.minTLSLevel {
		return module.TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS it not available or unauthenticated but required",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}
	return module.TLSNone, nil
}

func init() {
	module.Register("mx_auth.mtasts", NewMTASTSPolicy)
	module.Register("mx_auth.sts_preload", NewSTSPreload)
	module.Register("mx_auth.dnssec", NewDNSSECPolicy)
	module.Register("mx_auth.dane", NewDANEPolicy)
	module.Register("mx_auth.local_policy", NewLocalPolicy)
}
