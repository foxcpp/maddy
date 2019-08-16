package maddy

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
	"strings"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"github.com/emersion/maddy/mtasts"
)

var ErrTLSRequired = errors.New("TLS is required for outgoing connections but target server doesn't support STARTTLS")

type RemoteTarget struct {
	name       string
	hostname   string
	requireTLS bool

	resolver Resolver

	mtastsCache        mtasts.Cache
	stsCacheUpdateTick *time.Ticker
	stsCacheUpdateDone chan struct{}

	Log log.Logger
}

var _ module.DeliveryTarget = &RemoteTarget{}

func NewRemoteTarget(_, instName string) (module.Module, error) {
	return &RemoteTarget{
		name:        instName,
		resolver:    net.DefaultResolver,
		mtastsCache: mtasts.Cache{Resolver: net.DefaultResolver},
		Log:         log.Logger{Name: "remote"},

		stsCacheUpdateDone: make(chan struct{}),
	}, nil
}

func (rt *RemoteTarget) Init(cfg *config.Map) error {
	cfg.String("hostname", true, true, "", &rt.hostname)
	cfg.String("mtasts_cache", false, false, filepath.Join(StateDirectory(cfg.Globals), "mtasts-cache"), &rt.mtastsCache.Location)
	cfg.Bool("debug", true, &rt.Log.Debug)
	cfg.Bool("require_tls", false, &rt.requireTLS)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if !filepath.IsAbs(rt.mtastsCache.Location) {
		rt.mtastsCache.Location = filepath.Join(StateDirectory(cfg.Globals), rt.mtastsCache.Location)
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

func (rt *RemoteTarget) Close() error {
	rt.stsCacheUpdateDone <- struct{}{}
	<-rt.stsCacheUpdateDone
	return nil
}

func (rt *RemoteTarget) Name() string {
	return "remote"
}

func (rt *RemoteTarget) InstanceName() string {
	return rt.name
}

type remoteConnection struct {
	recipients []string
	serverName string
	*smtp.Client
}

type remoteDelivery struct {
	rt       *RemoteTarget
	mailFrom string
	ctx      *module.DeliveryContext

	connections map[string]*remoteConnection
}

func (rt *RemoteTarget) Start(ctx *module.DeliveryContext, mailFrom string) (module.Delivery, error) {
	return &remoteDelivery{
		rt:          rt,
		mailFrom:    mailFrom,
		ctx:         ctx,
		connections: map[string]*remoteConnection{},
	}, nil
}

func (rd *remoteDelivery) AddRcpt(to string) error {
	_, domain, err := splitAddress(to)
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
		rd.rt.Log.Printf("RCPT TO failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
		return err
	}

	conn.recipients = append(conn.recipients, to)
	return nil
}

func (rd *remoteDelivery) Body(header textproto.Header, b module.Buffer) error {
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
				rd.rt.Log.Printf("DATA failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
				errCh <- err
				return
			}
			bodyR, err := b.Open()
			if err != nil {
				rd.rt.Log.Printf("failed to open body buffer: %v (delivery ID = %s)", err, rd.ctx.DeliveryID)
				errCh <- err
				return
			}
			if err = textproto.WriteHeader(bodyW, header); err != nil {
				rd.rt.Log.Printf("header write failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
				errCh <- err
				return
			}
			if _, err = io.Copy(bodyW, bodyR); err != nil {
				rd.rt.Log.Printf("body write failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
				errCh <- err
				return
			}

			if err := bodyW.Close(); err != nil {
				rd.rt.Log.Printf("body write final failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
				errCh <- err
				return
			}

			errCh <- nil
		}()
	}

	// TODO: Report partial errors early for LMTP. See github.com/emersion/go-smtp/pull/56

	partialErr := PartialError{
		Errs: map[string]error{},
	}
	for domain, conn := range rd.connections {
		err := <-errChans[domain]
		if err != nil {
			if isTemporaryErr(err) {
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
		rd.rt.Log.Debugf("disconnected from %s (delivery ID = %s)", conn.serverName, rd.ctx.DeliveryID)
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

	stsPolicy, err := rd.rt.getSTSPolicy(domain)
	if err != nil {
		return nil, err
	}

	var lastErr error
	conn := &remoteConnection{}
	for _, addr := range addrs {
		if stsPolicy != nil && !stsPolicy.Match(addr) {
			rd.rt.Log.Printf("ignoring MX record for %s, as it is not matched by MTS-STS stsPolicy (%v) (delivery ID = %s)", addr, stsPolicy.MX, rd.ctx.DeliveryID)
			lastErr = ErrNoMXMatchedBySTS
			continue
		}
		conn.serverName = addr

		conn.Client, err = connectToServer(rd.rt.hostname, addr, rd.rt.requireTLS)
		if err != nil {
			rd.rt.Log.Debugf("failed to connect to %s: %v (delivery ID = %s)", addr, err, rd.ctx.DeliveryID)
			lastErr = err
			continue
		}
	}
	if conn.Client == nil {
		rd.rt.Log.Printf("no usable MX servers found for %s, last error (%s): %v (delivery ID = %s)", domain, conn.serverName, lastErr, rd.ctx.DeliveryID)
		return nil, lastErr
	}

	if err := conn.Mail(rd.mailFrom); err != nil {
		rd.rt.Log.Printf("MAIL FROM failed: %v (server = %s, delivery ID = %s)", err, conn.serverName, rd.ctx.DeliveryID)
		return nil, err
	}

	rd.rt.Log.Debugf("connected to %s (delivery ID = %s)", conn.serverName, rd.ctx.DeliveryID)
	rd.connections[domain] = conn

	return conn, nil
}

func (rt *RemoteTarget) getSTSPolicy(domain string) (*mtasts.Policy, error) {
	stsPolicy, err := rt.mtastsCache.Get(domain)
	if err != nil && err != mtasts.ErrNoPolicy {
		rt.Log.Printf("failed to fetch MTA-STS policy for %s: %v", domain, err)
		// TODO: Problems with policy should be treated as temporary ones.
		return nil, err
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

func (rt *RemoteTarget) stsCacheUpdater() {
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

func (rt *RemoteTarget) lookupTargetServers(domain string) ([]string, error) {
	records, err := rt.resolver.LookupMX(context.Background(), domain)
	if err != nil {
		return nil, err
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

func isTemporaryErr(err error) bool {
	if protoErr, ok := err.(*nettextproto.Error); ok {
		return (protoErr.Code / 100) == 4
	}
	if smtpErr, ok := err.(*smtp.SMTPError); ok {
		return (smtpErr.Code / 100) == 4
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary()
	}

	if err == ErrTLSRequired {
		return false
	}

	// Connection error? Assume it is temporary.
	return true
}

func init() {
	module.Register("remote", NewRemoteTarget)
}
