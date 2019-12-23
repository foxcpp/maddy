package remote

import (
	"context"
	"crypto/tls"
	"os"
	"time"

	"github.com/foxcpp/go-mtasts"
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
		// tlsLevel contains the TLS security level estabilished by checks
		// executed before.
		//
		// domain is passed to the CheckConn to allow simpler implementation
		// of stateless policy objects.
		//
		// If tlsState.HandshakeCompleted is false, TLS is not used. If
		// tlsState.VerifiedChains is nil, InsecureSkipVerify was used (no
		// ServerName or PKI check was done).
		CheckConn(ctx context.Context, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error)

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
		updaterStop: make(chan struct{}),
		log:         log.Logger{Name: "remote/mtasts", Debug: debug},
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

	go c.updater()

	return c, nil
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
	c.updaterStop <- struct{}{}
	<-c.updaterStop
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

func (c *mtastsDelivery) CheckConn(ctx context.Context, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
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

func (dnssecPolicy) CheckConn(ctx context.Context, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
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

func (c *daneDelivery) CheckConn(ctx context.Context, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
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

func (l localPolicy) CheckConn(ctx context.Context, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error) {
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
			policy, err = NewMTASTSPolicy(rt.resolver, rt.Log.Debug, config.NewMap(m.Globals, node))
		case "dane":
			if node.Children != nil {
				return nil, m.MatchErr("policy offers no additional configuration")
			}
			policy = NewDANEPolicy(rt.extResolver, rt.Log.Debug)
		case "dnssec":
			if node.Children != nil {
				return nil, m.MatchErr("policy offers no additional configuration")
			}
			policy = &dnssecPolicy{}
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
		var (
			policy Policy
			err    error
		)
		switch name {
		case "mtasts":
			policy, err = NewMTASTSPolicy(rt.resolver, rt.Log.Debug, config.NewMap(globals, &config.Node{}))
		case "dane":
			policy = NewDANEPolicy(rt.extResolver, rt.Log.Debug)
		case "dnssec":
			policy = &dnssecPolicy{}
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
