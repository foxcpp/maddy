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
	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/future"
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
	"github.com/foxcpp/maddy/internal/target"
)

type (
	// Policy is an object that provides security check for outbound connections.
	// It can do one of the following:
	//
	// - Check effective TLS level or MX level against some configured or
	// discovered value.
	// E.g. local policy.
	//
	// - Raise the security level if certain condition about used MX or
	// connection is met.
	// E.g. DANE Policy raises TLS level to Authenticated is a matching
	// TLSA record is discovered.
	//
	// - Reject the connection if certain condition about used MX or
	// connection is _not_ met.
	// E.g. An enforced MTA-STS Policy rejects MX records not matching it.
	//
	// It is not recommended to mix different types of behavior described above
	// in the same implementation.
	// Specifically, the first type is used mostly for local policies and not
	// really practical.
	Policy interface {
		Start(*module.MsgMetadata) DeliveryPolicy
		Close() error
	}

	// DeliveryPolicy is an interface of per-delivery object that estabilishes
	// and verifies required and effective security for MX records and TLS
	// connections.
	DeliveryPolicy interface {
		// PrepareDomain is called before DNS MX lookup and may asynchronously
		// start additional lookups necessary for policy application in CheckMX
		// or CheckConn.
		//
		// If there any errors - they should be deferred to the CheckMX or
		// CheckConn call.
		PrepareDomain(ctx context.Context, domain string)

		// PrepareDomain is called before connection and may asynchronously
		// start additional lookups necessary for policy application in
		// CheckConn.
		//
		// If there any errors - they should be deferred to the CheckConn
		// call.
		PrepareConn(ctx context.Context, mx string)

		// CheckMX is called to check whether the policy permits to use a MX.
		//
		// mxLevel contains the MX security level estabilished by checks
		// executed before.
		//
		// domain is passed to the CheckMX to allow simpler implementation
		// of stateless policy objects.
		//
		// dnssec is true if the MX lookup was performed using DNSSEC-enabled
		// resolver and the zone is signed and its signature is valid.
		CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error)

		// CheckConn is called to check whether the policy permits to use this
		// connection.
		//
		// tlsLevel and mxLevel contain the TLS security level estabilished by
		// checks executed before.
		//
		// domain is passed to the CheckConn to allow simpler implementation
		// of stateless policy objects.
		//
		// If tlsState.HandshakeCompleted is false, TLS is not used. If
		// tlsState.VerifiedChains is nil, InsecureSkipVerify was used (no
		// ServerName or PKI check was done).
		CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error)

		// Reset cleans the internal object state for use with another message.
		// newMsg may be nil if object is not needed anymore.
		Reset(newMsg *module.MsgMetadata)
	}
)

type (
	mtastsPolicy struct {
		cache       *mtasts.Cache
		mtastsGet   func(context.Context, string) (*mtasts.Policy, error)
		updaterStop chan struct{}
		log         log.Logger
	}
	mtastsDelivery struct {
		c         *mtastsPolicy
		domain    string
		policyFut *future.Future
		log       log.Logger
	}
)

func NewMTASTSPolicy(r dns.Resolver, debug bool, cfg *config.Map) (*mtastsPolicy, error) {
	c := &mtastsPolicy{
		log: log.Logger{Name: "remote/mtasts", Debug: debug},
	}

	var (
		storeType string
		storeDir  string
	)
	cfg.Enum("cache", false, false, []string{"ram", "fs"}, "fs", &storeType)
	cfg.String("fs_dir", false, false, "mtasts_cache", &storeDir)
	if _, err := cfg.Process(); err != nil {
		return nil, err
	}

	switch storeType {
	case "fs":
		if err := os.MkdirAll(storeDir, os.ModePerm); err != nil {
			return nil, err
		}
		c.cache = mtasts.NewFSCache(storeDir)
	case "ram":
		c.cache = mtasts.NewRAMCache()
	default:
		panic("mtasts policy init: unknown cache type")
	}
	c.cache.Resolver = dns.DefaultResolver()
	c.mtastsGet = c.cache.Get

	return c, nil
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

func (c *mtastsPolicy) Start(msgMeta *module.MsgMetadata) DeliveryPolicy {
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

func (c *mtastsDelivery) CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error) {
	policyI, err := c.policyFut.GetContext(ctx)
	if err != nil {
		c.log.DebugMsg("MTA-STS error", "err", err)
		return MXNone, nil
	}
	policy := policyI.(*mtasts.Policy)

	if !policy.Match(mx) {
		if policy.Mode == mtasts.ModeEnforce {
			return MXNone, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Failed to estabilish the MX record authenticity (MTA-STS)",
			}
		}
		c.log.Msg("MX does not match published non-enforced MTA-STS policy", "mx", mx, "domain", c.domain)
		return MXNone, nil
	}
	return MX_MTASTS, nil
}

func (c *mtastsDelivery) CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
	policyI, err := c.policyFut.GetContext(ctx)
	if err != nil {
		c.c.log.DebugMsg("MTA-STS error", "err", err)
		return TLSNone, nil
	}
	policy := policyI.(*mtasts.Policy)

	if policy.Mode != mtasts.ModeEnforce {
		return TLSNone, nil
	}

	if !tlsState.HandshakeComplete {
		return TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS is required but unavailable or failed (MTA-STS)",
		}
	}

	if tlsState.VerifiedChains == nil {
		return TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message: "Recipient server TLS certificate is not trusted but " +
				"authentication is required by MTA-STS",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}

	return TLSNone, nil
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
	// This is adjusted for preloadUpdateGrace so we will the chance to make 10
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
	}
)

func NewSTSPreloadPolicy(debug bool, client *http.Client, listDownload FuncPreloadList, cfg *config.Map) (*stsPreloadPolicy, error) {
	p := &stsPreloadPolicy{
		log:          log.Logger{Name: "remote/preload", Debug: debug},
		client:       client,
		listDownload: preload.Download,
	}

	var sourcePath string
	cfg.String("source", false, false, "eff", &sourcePath)
	cfg.Bool("enforce_testing", false, true, &p.enforceTesting)
	if _, err := cfg.Process(); err != nil {
		return nil, err
	}

	var err error
	p.l, err = p.load(client, listDownload, sourcePath)
	if err != nil {
		return nil, err
	}

	p.sourcePath = sourcePath

	return p, nil
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
	case strings.HasPrefix(sourcePath, "http://"):
		// XXX: Only for testing, remove later.
		fallthrough
	case strings.HasPrefix(sourcePath, "https://"):
		// Download list using HTTPS.
		// TODO: Cache on disk and update it asynchronously to reduce start-up
		// time. This will also reduce persistent attacker ability to prevent
		// list (re-)discovery.
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
		// TODO: Increase update delay for multiple failures.

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

func (p *stsPreloadPolicy) Start(*module.MsgMetadata) DeliveryPolicy {
	return &preloadDelivery{stsPreloadPolicy: p}
}

func (p *preloadDelivery) Reset(*module.MsgMetadata)                        {}
func (p *preloadDelivery) PrepareDomain(ctx context.Context, domain string) {}
func (p *preloadDelivery) PrepareConn(ctx context.Context, mx string)       {}
func (p *preloadDelivery) CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error) {
	// MTA-STS policy was discovered and took effect already. Do not use
	// preload list.
	if mxLevel == MX_MTASTS {
		p.mtastsPresent = true
		return MXNone, nil
	}

	p.lLock.RLock()
	defer p.lLock.RUnlock()

	if p.l.Expired() {
		p.log.Msg("STARTTLS Everywhere list is expired, ignoring")
		return MXNone, nil
	}

	ent, ok := p.l.Lookup(domain)
	if !ok {
		p.log.DebugMsg("no entry", "domain", domain)
		return MXNone, nil
	}
	sts := ent.STS(p.l)
	if !sts.Match(mx) {
		if sts.Mode == mtasts.ModeEnforce || p.enforceTesting {
			return MXNone, &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
				Message:      "Failed to estabilish the MX record authenticity (STARTTLS Everywhere)",
			}
		}
		p.log.Msg("MX does not match published non-enforced STARTLS Everywhere entry", "mx", mx, "domain", domain)
		return MXNone, nil
	}
	p.log.DebugMsg("MX OK", "domain", domain)
	return MX_MTASTS, nil
}

func (p *preloadDelivery) CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
	// MTA-STS policy was discovered and took effect already. Do not use
	// preload list. We cannot check level for MX_MTASTS because we can set
	// it too in CheckMX.
	if p.mtastsPresent {
		return TLSNone, nil
	}

	p.lLock.RLock()
	defer p.lLock.RUnlock()

	if p.l.Expired() {
		p.log.Msg("STARTTLS Everywhere list is expired, ignoring")
		return TLSNone, nil
	}

	ent, ok := p.l.Lookup(domain)
	if !ok {
		p.log.DebugMsg("no entry", "domain", domain)
		return TLSNone, nil
	}
	if ent.Mode != mtasts.ModeEnforce && !p.enforceTesting {
		return TLSNone, nil
	}

	if !tlsState.HandshakeComplete {
		return TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS is required but unavailable or failed (STARTTLS Everywhere)",
		}
	}

	if tlsState.VerifiedChains == nil {
		return TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message: "Recipient server TLS certificate is not trusted but " +
				"authentication is required by STARTTLS Everywhere list",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}

	p.log.DebugMsg("TLS OK", "domain", domain)

	return TLSNone, nil
}

func (p *stsPreloadPolicy) Close() error {
	if p.updaterStop != nil {
		p.updaterStop <- struct{}{}
		<-p.updaterStop
		p.updaterStop = nil
	}
	return nil
}

type dnssecPolicy struct{}

func (dnssecPolicy) Start(*module.MsgMetadata) DeliveryPolicy {
	return dnssecPolicy{}
}

func (dnssecPolicy) Close() error {
	return nil
}

func (dnssecPolicy) Reset(*module.MsgMetadata)                        {}
func (dnssecPolicy) PrepareDomain(ctx context.Context, domain string) {}
func (dnssecPolicy) PrepareConn(ctx context.Context, mx string)       {}

func (dnssecPolicy) CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error) {
	if dnssec {
		return MX_DNSSEC, nil
	}
	return MXNone, nil
}

func (dnssecPolicy) CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
	return TLSNone, nil
}

type (
	danePolicy struct {
		extResolver *dns.ExtResolver
		log         log.Logger
	}
	daneDelivery struct {
		c       *danePolicy
		tlsaFut *future.Future
	}
)

func NewDANEPolicy(extR *dns.ExtResolver, debug bool) *danePolicy {
	return &danePolicy{
		log:         log.Logger{Name: "remote/dane", Debug: debug},
		extResolver: extR,
	}
}

func (c *danePolicy) Start(*module.MsgMetadata) DeliveryPolicy {
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

func (c *daneDelivery) CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error) {
	return MXNone, nil
}

func (c *daneDelivery) CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
	// No DNSSEC support.
	if c.c.extResolver == nil {
		return TLSNone, nil
	}

	recsI, err := c.tlsaFut.GetContext(ctx)
	if err != nil {
		// No records.
		if dns.IsNotFound(err) {
			return TLSNone, nil
		}

		// Lookup error here indicates a resolution failure or may also
		// indicate a bogus DNSSEC signature.
		// There is a big problem with differentiating these two.
		//
		// We assume DANE failure in both cases as a safety measure.
		// However, there is a possibility of a temporary error condition,
		// so we mark it as such.
		return TLSNone, exterrors.WithTemporary(err, true)
	}
	recs := recsI.([]dns.TLSA)

	overridePKIX, err := verifyDANE(recs, mx, tlsState)
	if err != nil {
		return TLSNone, err
	}
	if overridePKIX {
		return TLSAuthenticated, nil
	}
	return TLSNone, nil
}

func (c *daneDelivery) Reset(*module.MsgMetadata) {}

type (
	localPolicy struct {
		minTLSLevel TLSLevel
		minMXLevel  MXLevel
	}
)

func NewLocalPolicy(cfg *config.Map) (localPolicy, error) {
	l := localPolicy{}

	var (
		minTLSLevel string
		minMXLevel  string
	)

	cfg.Enum("min_tls_level", false, false,
		[]string{"none", "encrypted", "authenticated"}, "encrypted", &minTLSLevel)
	cfg.Enum("min_mx_level", false, false,
		[]string{"none", "mtasts", "dnssec"}, "none", &minMXLevel)
	if _, err := cfg.Process(); err != nil {
		return localPolicy{}, err
	}

	// Enum checks the value against allowed list, no 'default' necessary.
	switch minTLSLevel {
	case "none":
		l.minTLSLevel = TLSNone
	case "encrypted":
		l.minTLSLevel = TLSEncrypted
	case "authenticated":
		l.minTLSLevel = TLSAuthenticated
	}
	switch minMXLevel {
	case "none":
		l.minMXLevel = MXNone
	case "mtasts":
		l.minMXLevel = MX_MTASTS
	case "dnssec":
		l.minMXLevel = MX_DNSSEC
	}

	return l, nil
}

func (l localPolicy) Start(msgMeta *module.MsgMetadata) DeliveryPolicy {
	return l
}

func (l localPolicy) Close() error {
	return nil
}

func (l localPolicy) Reset(*module.MsgMetadata)                        {}
func (l localPolicy) PrepareDomain(ctx context.Context, domain string) {}
func (l localPolicy) PrepareConn(ctx context.Context, mx string)       {}

func (l localPolicy) CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error) {
	if mxLevel < l.minMXLevel {
		return MXNone, &exterrors.SMTPError{
			// Err on the side of caution if policy evaluation was messed up by
			// a temporary error (we can't know with the current design).
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 0},
			Message:      "Failed to estabilish the MX record authenticity",
			Misc: map[string]interface{}{
				"mx_level": mxLevel,
			},
		}
	}
	return MXNone, nil
}

func (l localPolicy) CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
	if tlsLevel < l.minTLSLevel {
		return TLSNone, &exterrors.SMTPError{
			Code:         451,
			EnhancedCode: exterrors.EnhancedCode{4, 7, 1},
			Message:      "TLS it not available or unauthenticated but required",
			Misc: map[string]interface{}{
				"tls_level": tlsLevel,
			},
		}
	}
	return TLSNone, nil
}

func (rt *Target) policyMatcher(name string) func(*config.Map, *config.Node) (interface{}, error) {
	return func(m *config.Map, node *config.Node) (interface{}, error) {
		// TODO: Fix rt.Log.Debug propagation. This function is called in
		// arbitrary order before or after the 'debug' directive is handled.
		switch len(node.Args) {
		case 0:
		case 1:
			if node.Args[0] != "off" {
				return nil, m.MatchErr("only 'off' argument is allowed")
			}
			return nil, nil
		default:
			return nil, m.MatchErr("at most one argument allowed")
		}

		var (
			policy Policy
			err    error
		)
		switch name {
		case "mtasts":
			policy, err = NewMTASTSPolicy(rt.resolver, log.DefaultLogger.Debug, config.NewMap(m.Globals, node))
		case "dane":
			if node.Children != nil {
				return nil, m.MatchErr("policy offers no additional configuration")
			}
			policy = NewDANEPolicy(rt.extResolver, log.DefaultLogger.Debug)
		case "dnssec":
			if node.Children != nil {
				return nil, m.MatchErr("policy offers no additional configuration")
			}
			policy = &dnssecPolicy{}
		case "sts_preload":
			policy, err = NewSTSPreloadPolicy(log.DefaultLogger.Debug, http.DefaultClient, preload.Download,
				config.NewMap(m.Globals, node))
		case "local":
			policy, err = NewLocalPolicy(config.NewMap(m.Globals, node))
		default:
			panic("unknown policy module")
		}
		if err != nil {
			return nil, err
		}

		return policy, nil
	}
}

func (rt *Target) defaultPolicy(globals map[string]interface{}, name string) func() (interface{}, error) {
	return func() (interface{}, error) {
		// TODO: Fix rt.Log.Debug propagation. This function is called in
		// arbitrary order before or after the 'debug' directive is handled.
		var (
			policy Policy
			err    error
		)
		switch name {
		case "mtasts":
			mtastsPol, err := NewMTASTSPolicy(rt.resolver, log.DefaultLogger.Debug, config.NewMap(globals, &config.Node{}))
			if err != nil {
				return nil, err
			}
			mtastsPol.StartUpdater()
			policy = mtastsPol
		case "dane":
			policy = NewDANEPolicy(rt.extResolver, log.DefaultLogger.Debug)
		case "dnssec":
			policy = &dnssecPolicy{}
		case "sts_preload":
			preloadPolicy, err := NewSTSPreloadPolicy(log.DefaultLogger.Debug, http.DefaultClient, preload.Download,
				config.NewMap(globals, &config.Node{}))
			if err != nil {
				return nil, err
			}
			preloadPolicy.StartUpdater()
			policy = preloadPolicy
		case "local":
			policy, err = NewLocalPolicy(config.NewMap(globals, &config.Node{}))
		default:
			panic("unknown policy module")
		}
		if err != nil {
			return nil, err
		}
		return policy, nil
	}
}
