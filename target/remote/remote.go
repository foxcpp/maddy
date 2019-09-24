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
	nettextproto "net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	"github.com/foxcpp/maddy/target/queue"
)

var ErrTLSRequired = errors.New("TLS is required for outgoing connections but target server doesn't support STARTTLS")

type Target struct {
	name       string
	hostname   string
	requireTLS bool

	resolver dns.Resolver

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
	cfg.String("hostname", true, true, "", &rt.hostname)
	cfg.String("mtasts_cache", false, false, filepath.Join(config.StateDirectory, "mtasts-cache"), &rt.mtastsCache.Location)
	cfg.Bool("debug", true, false, &rt.Log.Debug)
	cfg.Bool("require_tls", false, false, &rt.requireTLS)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if !filepath.IsAbs(rt.mtastsCache.Location) {
		rt.mtastsCache.Location = filepath.Join(config.StateDirectory, rt.mtastsCache.Location)
	}
	if err := os.MkdirAll(rt.mtastsCache.Location, os.ModePerm); err != nil {
		return err
	}
	rt.mtastsCache.Logger = &rt.Log
	// MTA-STS policies typically have max_age around one day, so updating them
	// twice a day should keep them up-to-date most of the time.
	rt.stsCacheUpdateTick = time.NewTicker(12 * time.Hour)
	go rt.stsCacheUpdater()

	return nil
}

func (rt *Target) Close() error {
	rt.stsCacheUpdateDone <- struct{}{}
	<-rt.stsCacheUpdateDone
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
		return toSMTPErr(err)
	}

	rd.recipients = append(rd.recipients, to)
	conn.recipients = append(conn.recipients, to)
	return nil
}

func (rd *remoteDelivery) Body(header textproto.Header, b buffer.Buffer) error {
	if rd.msgMeta.Quarantine {
		return exterrors.WithTemporary(errors.New("remote: refusing to deliver quarantined message"), false)
	}

	errChans := make(map[string]chan error, len(rd.connections))
	for domain := range rd.connections {
		errChans[domain] = make(chan error)
	}

	for i, conn := range rd.connections {
		errCh := errChans[i]
		conn := conn
		go func() {
			bodyW, err := conn.Data()
			if err != nil {
				rd.Log.Printf("DATA failed: %v (server = %s)", err, conn.serverName)
				errCh <- toSMTPErr(err)
				return
			}
			bodyR, err := b.Open()
			if err != nil {
				rd.Log.Printf("failed to open body buffer: %v", err)
				errCh <- toSMTPErr(err)
				return
			}
			if err = textproto.WriteHeader(bodyW, header); err != nil {
				rd.Log.Printf("header write failed: %v (server = %s)", err, conn.serverName)
				errCh <- toSMTPErr(err)
				return
			}
			if _, err = io.Copy(bodyW, bodyR); err != nil {
				rd.Log.Printf("body write failed: %v (server = %s)", err, conn.serverName)
				errCh <- toSMTPErr(err)
				return
			}

			if err := bodyW.Close(); err != nil {
				rd.Log.Printf("body write final failed: %v (server = %s)", err, conn.serverName)
				errCh <- toSMTPErr(err)
				return
			}

			errCh <- nil
		}()
	}

	// TODO: Report partial errors early for LMTP. See github.com/emersion/go-smtp/pull/56

	partialErr := queue.PartialError{
		Errs: map[string]error{},
	}
	for domain, conn := range rd.connections {
		err := <-errChans[domain]
		if err != nil {
			if exterrors.IsTemporary(err) {
				partialErr.TemporaryFailed = append(partialErr.TemporaryFailed, conn.recipients...)
			} else {
				partialErr.Failed = append(partialErr.Failed, conn.recipients...)
			}
			for _, rcpt := range conn.recipients {
				partialErr.Errs[rcpt] = err
			}
		}
	}

	if len(partialErr.Errs) == 0 {
		return nil
	}
	return partialErr
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

	addrs, err := rd.rt.lookupTargetServers(domain)
	if err != nil {
		return nil, err
	}

	if rd.rt.requireTLS {
		rd.Log.Debugf("TLS is required for %s due to a local configuration", domain)
	}

	stsPolicy, err := rd.rt.getSTSPolicy(domain)
	if err != nil {
		return nil, err
	}
	if stsPolicy != nil {
		rd.Log.Debugf("TLS is required for %s due to a MTA-STS policy", domain)
	}

	requireTLS := rd.rt.requireTLS || stsPolicy != nil
	if !requireTLS {
		rd.Log.Printf("TLS is not enforced when delivering to %s", domain)
	}

	var lastErr error
	conn := &remoteConnection{}
	for _, addr := range addrs {
		if stsPolicy != nil && !stsPolicy.Match(addr) {
			rd.Log.Printf("ignoring MX record for %s, as it is not matched by MTS-STS stsPolicy (%v)",
				addr, stsPolicy.MX)
			lastErr = ErrNoMXMatchedBySTS
			continue
		}
		conn.serverName = addr

		conn.Client, err = connectToServer(rd.rt.hostname, addr, requireTLS)
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
		return nil, toSMTPErr(err)
	}

	rd.Log.Debugf("connected to %s", conn.serverName)
	rd.connections[domain] = conn

	return conn, nil
}

func (rt *Target) getSTSPolicy(domain string) (*mtasts.Policy, error) {
	stsPolicy, err := rt.mtastsCache.Get(domain)
	if err != nil && err != mtasts.ErrNoPolicy {
		rt.Log.Printf("failed to fetch MTA-STS policy for %s: %v", domain, err)
		// TODO: Problems with policy should be treated as temporary ones.
		return nil, exterrors.WithTemporary(err, true)
	}
	if stsPolicy != nil && stsPolicy.Mode != mtasts.ModeEnforce {
		// Throw away policy if it is not enforced, we don't do TLSRPT for now.
		// TODO: TLS reporting.
		rt.Log.Debugf("ignoring non-enforced MTA-STS policy for %s", domain)
		return nil, nil
	}
	return stsPolicy, nil
}

var ErrNoMXMatchedBySTS = errors.New("remote: no MX record matched MTA-STS policy")

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

func connectToServer(ourHostname, address string, requireTLS bool) (*smtp.Client, error) {
	cl, err := smtp.Dial(address + ":25")
	if err != nil {
		return nil, err
	}

	if err := cl.Hello(ourHostname); err != nil {
		return nil, err
	}

	if tlsOk, _ := cl.Extension("STARTTLS"); tlsOk {
		if err := cl.StartTLS(&tls.Config{
			ServerName: address,
		}); err != nil {
			return nil, err
		}
	} else if requireTLS {
		return nil, ErrTLSRequired
	}

	return cl, nil
}

func (rt *Target) lookupTargetServers(domain string) ([]string, error) {
	records, err := rt.resolver.LookupMX(context.Background(), domain)
	if err != nil {
		return nil, err
	}

	// Fallback to A/AAA RR when no MX records are present as
	// required by RFC 5321 Section 5.1.
	if len(records) == 0 {
		records = append(records, &net.MX{
			Host: domain,
			Pref: 0,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})

	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, record.Host)
	}
	return hosts, nil
}

// TODO: Remove when emersion/go-smtp#64 gets merged.

func parseEnhancedCode(s string) (smtp.EnhancedCode, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return smtp.EnhancedCode{}, fmt.Errorf("wrong amount of enhanced code parts")
	}

	code := smtp.EnhancedCode{}
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return code, err
		}
		code[i] = num
	}
	return code, nil
}

func toSMTPErr(originalErr error) error {
	protoErr, ok := originalErr.(*nettextproto.Error)
	if !ok {
		return originalErr
	}

	smtpErr := &smtp.SMTPError{
		Code:    protoErr.Code,
		Message: protoErr.Msg,
	}

	parts := strings.SplitN(protoErr.Msg, " ", 2)
	if len(parts) != 2 {
		return exterrors.WithTemporary(smtpErr, smtpErr.Code/100 == 4)
	}

	enchCode, err := parseEnhancedCode(parts[0])
	if err != nil {
		return exterrors.WithTemporary(smtpErr, smtpErr.Code/100 == 4)
	}

	smtpErr.EnhancedCode = enchCode
	smtpErr.Message = parts[1]
	// TODO: This will not need this wrapping when emersion/go-smtp#65 gets merged.
	return exterrors.WithTemporary(smtpErr, smtpErr.Code/100 == 4)
}

func init() {
	module.Register("remote", New)
}
