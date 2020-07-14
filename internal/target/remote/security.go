package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/foxcpp/go-mtasts"
	"github.com/foxcpp/go-mtasts/preload"
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
				Message:      "Failed to estabilish the module.MX record authenticity (MTA-STS)",
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

var (
	// Delay between list expiry and first update attempt.
	preloadUpdateGrace = 5 * time.Minute

	// Minimal time between preload list update attempts.
	//
	// This is adjusted for preloadUpdateGrace so we will have the chance to make 10
	// attempts to update list before it expires.
	preloadUpdateCooldown = 30 * time.Second
)

type (
	FuncPreloadList  = func(*http.Client, preload.Source) (*preload.List, error)
	stsPreloadPolicy struct {
		l     *preload.List
		lLock sync.RWMutex
		log   log.Logger

		updaterStop chan struct{}

		sourcePath     string
		client         *http.Client
		listDownload   FuncPreloadList
		enforceTesting bool

		instName string
	}
)

func NewSTSPreload(_, instName string, _, _ []string) (module.Module, error) {
	return &stsPreloadPolicy{
		instName:     instName,
		client:       http.DefaultClient,
		listDownload: preload.Download,
		log:          log.Logger{Name: "mx_auth.sts_preload", Debug: log.DefaultLogger.Debug},
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
	var sourcePath string
	cfg.String("source", false, false, "eff", &sourcePath)
	cfg.Bool("enforce_testing", false, true, &c.enforceTesting)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	var err error
	c.l, err = c.load(c.client, c.listDownload, sourcePath)
	if err != nil {
		return err
	}

	c.sourcePath = sourcePath

	return nil
}

func (p *stsPreloadPolicy) load(client *http.Client, listDownload FuncPreloadList, sourcePath string) (*preload.List, error) {
	var (
		l   *preload.List
		err error
	)
	src := preload.Source{
		ListURI: sourcePath,
	}
	switch {
	case strings.HasPrefix(sourcePath, "file://"):
		// Load list from FS.
		path := strings.TrimPrefix(sourcePath, "file://")
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		// If the list is provided by an external FS source, it is its
		// responsibility to make sure it is valid. Hard-fail if it is not.
		l, err = preload.Read(f)
		if err != nil {
			return nil, fmt.Errorf("remote/preload: %w", err)
		}
		defer f.Close()

		p.log.DebugMsg("loaded list from FS", "entries", len(l.Policies), "path", path)
	case sourcePath == "eff":
		src = preload.STARTTLSEverywhere
		fallthrough
	case strings.HasPrefix(sourcePath, "https://"):
		// Download list using HTTPS.
		// TODO(GH #207): Cache on disk and update it asynchronously to reduce start-up
		// time. This will also reduce persistent attacker ability to prevent
		// list (re-)discovery
		p.log.DebugMsg("downloading list", "uri", sourcePath)

		l, err = listDownload(client, src)
		if err != nil {
			return nil, fmt.Errorf("remote/preload: %w", err)
		}

		p.log.DebugMsg("downloaded list", "entries", len(l.Policies), "uri", sourcePath)
	default:
		return nil, fmt.Errorf("remote/preload: unknown list source or unsupported schema: %v", sourcePath)
	}

	return l, nil
}

// StartUpdater starts a goroutine to update the used list periodically until Close is
// called.
//
// It can be called only once per stsPreloadPolicy instance.
func (p *stsPreloadPolicy) StartUpdater() {
	p.updaterStop = make(chan struct{})
	go p.updater()
}

func (p *stsPreloadPolicy) updater() {
	for {
		updateDelay := time.Until(time.Time(p.l.Expires)) - preloadUpdateGrace
		if updateDelay <= 0 {
			updateDelay = preloadUpdateCooldown
		}
		// TODO(GH #210): Increase update delay for multiple failures.

		t := time.NewTimer(updateDelay)

		p.log.DebugMsg("sleeping to update", "delay", updateDelay)

		select {
		case <-t.C:
			// Attempt to update the list.
			newList, err := p.load(p.client, p.listDownload, p.sourcePath)
			if err != nil {
				p.log.Error("failed to update list", err)
				continue
			}
			if err := p.update(newList); err != nil {
				p.log.Error("failed to update list", err)
				continue
			}
			p.log.DebugMsg("updated list", "entries", len(newList.Policies))
		case <-p.updaterStop:
			t.Stop()
			p.updaterStop <- struct{}{}
			return
		}
	}
}

func (p *stsPreloadPolicy) update(newList *preload.List) error {
	if newList.Expired() {
		return exterrors.WithFields(errors.New("the new STARTLS Everywhere list is expired"),
			map[string]interface{}{
				"timestamp": newList.Timestamp,
				"expires":   newList.Expires,
			})
	}

	p.lLock.Lock()
	defer p.lLock.Unlock()

	if time.Time(newList.Timestamp).Before(time.Time(p.l.Timestamp)) {
		return exterrors.WithFields(errors.New("the new list is older than the currently used one"),
			map[string]interface{}{
				"old_timestamp": p.l.Timestamp,
				"new_timestamp": newList.Timestamp,
				"expires":       newList.Expires,
			})
	}

	p.l = newList

	return nil
}

type preloadDelivery struct {
	*stsPreloadPolicy
	mtastsPresent bool
}

func (p *stsPreloadPolicy) Start(*module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return &preloadDelivery{stsPreloadPolicy: p}
}

func (p *preloadDelivery) Reset(*module.MsgMetadata)                        {}
func (p *preloadDelivery) PrepareDomain(ctx context.Context, domain string) {}
func (p *preloadDelivery) PrepareConn(ctx context.Context, mx string)       {}
func (p *preloadDelivery) CheckMX(ctx context.Context, mxLevel module.MXLevel, domain, mx string, dnssec bool) (module.MXLevel, error) {
	// MTA-STS policy was discovered and took effect already. Do not use
	// preload list.
	if mxLevel == module.MX_MTASTS {
		p.mtastsPresent = true
		return module.MXNone, nil
	}

	p.lLock.RLock()
	defer p.lLock.RUnlock()

	if p.l.Expired() {
		p.log.Msg("STARTTLS Everywhere list is expired, ignoring")
		return module.MXNone, nil
	}

	ent, ok := p.l.Lookup(domain)
	if !ok {
		p.log.DebugMsg("no entry", "domain", domain)
		return module.MXNone, nil
	}
	sts := ent.STS(p.l)
	if !sts.Match(mx) {
		if sts.Mode == mtasts.ModeEnforce || p.enforceTesting {
			return module.MXNone, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Failed to estabilish the module.MX record authenticity (STARTTLS Everywhere)",
			}
		}
		p.log.Msg("MX does not match published non-enforced STARTLS Everywhere entry", "mx", mx, "domain", domain)
		return module.MXNone, nil
	}
	p.log.DebugMsg("MX OK", "domain", domain)
	return module.MX_MTASTS, nil
}

func (p *preloadDelivery) CheckConn(ctx context.Context, mxLevel module.MXLevel, tlsLevel module.TLSLevel, domain, mx string, tlsState tls.ConnectionState) (module.TLSLevel, error) {
	// MTA-STS policy was discovered and took effect already. Do not use
	// preload list. We cannot check level for module.MX_MTASTS because we can set
	// it too in CheckMX.
	if p.mtastsPresent {
		return module.TLSNone, nil
	}

	p.lLock.RLock()
	defer p.lLock.RUnlock()

	if p.l.Expired() {
		p.log.Msg("STARTTLS Everywhere list is expired, ignoring")
		return module.TLSNone, nil
	}

	ent, ok := p.l.Lookup(domain)
	if !ok {
		p.log.DebugMsg("no entry", "domain", domain)
		return module.TLSNone, nil
	}
	if ent.Mode != mtasts.ModeEnforce && !p.enforceTesting {
		return module.TLSNone, nil
	}

	if !tlsState.HandshakeComplete {
		return module.TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS is required but unavailable or failed (STARTTLS Everywhere)",
		}
	}

	if tlsState.VerifiedChains == nil {
		return module.TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message: "Recipient server module.TLS certificate is not trusted but " +
				"authentication is required by STARTTLS Everywhere list",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}

	p.log.DebugMsg("TLS OK", "domain", domain)

	return module.TLSNone, nil
}

func (p *stsPreloadPolicy) Close() error {
	if p.updaterStop != nil {
		p.updaterStop <- struct{}{}
		<-p.updaterStop
		p.updaterStop = nil
	}
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

	_, err = cfg.Process() // will fail if there is any directive
	return err
}

func (c *danePolicy) Start(*module.MsgMetadata) module.DeliveryMXAuthPolicy {
	return &daneDelivery{c: c}
}

func (c *danePolicy) Close() error {
	return nil
}

func (c *daneDelivery) PrepareDomain(ctx context.Context, domain string) {}

func (c *daneDelivery) PrepareConn(ctx context.Context, mx string) {
	// No DNSSEC support.
	if c.c.extResolver == nil {
		return
	}

	c.tlsaFut = future.New()

	go func() {
		ad, recs, err := c.c.extResolver.AuthLookupTLSA(ctx, "25", "tcp", mx)
		if err != nil {
			c.tlsaFut.Set([]dns.TLSA{}, err)
			return
		}
		if !ad {
			// Per https://tools.ietf.org/html/rfc7672#section-2.2 we interpret
			// a non-authenticated RRset just like an empty RRset. Side note:
			// "bogus" signatures are expected to be caught by the upstream
			// resolver.
			c.tlsaFut.Set([]dns.TLSA{}, err)
			return
		}

		// recs can be empty indicating absence of records.

		c.tlsaFut.Set(recs, err)
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

	overridePKIX, err := verifyDANE(recs, mx, tlsState)
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
			Message:      "Failed to estabilish the module.MX record authenticity",
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
