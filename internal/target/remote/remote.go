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
	"github.com/foxcpp/go-mtasts"
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
	name        string
	hostname    string
	minTLSLevel TLSLevel
	minMXLevel  MXLevel
	tlsConfig   *tls.Config

	resolver    dns.Resolver
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
	extResolver *dns.ExtResolver

	// This is the callback set to mock mtasts.Cache in tests.
	// It is nil in real code, mtastsCache should be used in this case.
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
	var (
		minTLSLevel string
		minMXLevel  string
		mtastsCache string
	)

	cfg.String("hostname", true, true, "", &rt.hostname)
	cfg.String("mtasts_cache", false, false, filepath.Join(config.StateDirectory, "mtasts-cache"), &mtastsCache)
	cfg.Bool("debug", true, false, &rt.Log.Debug)
	cfg.Enum("min_tls_level", false, false,
		[]string{"none", "encrypted", "authenticated"}, "encrypted", &minTLSLevel)
	cfg.Enum("min_mx_level", false, false,
		[]string{"none", "mtasts", "dnssec"}, "none", &minMXLevel)
	cfg.Custom("tls_client", true, false, func() (interface{}, error) {
		return &tls.Config{}, nil
	}, config.TLSClientBlock, &rt.tlsConfig)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	rt.mtastsCache = *mtasts.NewFSCache(mtastsCache)

	switch minTLSLevel {
	case "none":
		rt.minTLSLevel = TLSNone
	case "encrypted":
		rt.minTLSLevel = TLSEncrypted
	case "authenticated":
		rt.minTLSLevel = TLSAuthenticated
	}
	switch minMXLevel {
	case "none":
		rt.minMXLevel = MXNone
	case "mtasts":
		rt.minMXLevel = MX_MTASTS
	case "dnssec":
		rt.minMXLevel = MX_DNSSEC
	}

	// INTERNATIONALIZATION: See RFC 6531 Section 3.7.1.
	var err error
	rt.hostname, err = idna.ToASCII(rt.hostname)
	if err != nil {
		return fmt.Errorf("remote: cannot represent the hostname as an A-label name: %w", err)
	}

	if err := rt.initMXAuth(mtastsCache); err != nil {
		return err
	}

	return nil
}

func (rt *Target) initMXAuth(mtastsDir string) error {
	var err error
	rt.extResolver, err = dns.NewExtResolver()
	if err != nil {
		if rt.minMXLevel >= MX_DNSSEC {
			return fmt.Errorf("remote: failed to init DNSSEC-aware stub resolver: %w", err)
		} else {
			rt.Log.Error("failed to initialize DNSSEC-aware stub resolver", err)
		}
	}

	if err := os.MkdirAll(mtastsDir, os.ModePerm); err != nil {
		return err
	}
	rt.mtastsCache.Resolver = rt.resolver
	// MTA-STS policies typically have max_age around one day, so updating them
	// twice a day should keep them up-to-date most of the time.
	rt.stsCacheUpdateTick = time.NewTicker(12 * time.Hour)
	rt.mtastsGet = rt.mtastsCache.Get
	go rt.stsCacheUpdater()

	return nil
}

func (rt *Target) Close() error {
	if rt.mtastsGet == nil {
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

	if rd.rt.minMXLevel < MX_DNSSEC {
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

	if rd.rt.extResolver == nil {
		return "", &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 1, 2},
			Message:      "Cannot authenticate the recipient IP",
			TargetName:   "remote",
			Reason:       "DNSSEC status is not available",
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
