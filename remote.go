package maddy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/emersion/maddy/config"
	"github.com/emersion/maddy/log"
	"github.com/emersion/maddy/module"
	"github.com/emersion/maddy/mtasts"
)

var ErrTLSRequired = errors.New("TLS is required for outgoing connections but target server doesn't support STARTTLS")

type RemoteDelivery struct {
	name       string
	hostname   string
	requireTLS bool

	mtastsCache        mtasts.Cache
	stsCacheUpdateTick *time.Ticker
	stsCacheUpdateDone chan struct{}

	Log log.Logger
}

func NewRemoteDelivery(_, instName string) (module.Module, error) {
	return &RemoteDelivery{
		name: instName,
		Log:  log.Logger{Name: "remote"},

		stsCacheUpdateDone: make(chan struct{}),
	}, nil
}

func (rd *RemoteDelivery) Init(cfg *config.Map) error {
	cfg.String("hostname", true, true, "", &rd.hostname)
	cfg.String("mtasts_cache", false, false, filepath.Join(StateDirectory(cfg.Globals), "mtasts-cache"), &rd.mtastsCache.Location)
	cfg.Bool("debug", true, &rd.Log.Debug)
	cfg.Bool("require_tls", false, &rd.requireTLS)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if !filepath.IsAbs(rd.mtastsCache.Location) {
		rd.mtastsCache.Location = filepath.Join(StateDirectory(cfg.Globals), rd.mtastsCache.Location)
	}
	if err := os.MkdirAll(rd.mtastsCache.Location, os.ModePerm); err != nil {
		return err
	}
	rd.mtastsCache.Logger = &rd.Log
	// MTA-STS policies typically have max_age around one day, so updating them
	// twice a day should keep them up-to-date most of the time.
	rd.stsCacheUpdateTick = time.NewTicker(12 * time.Hour)
	go rd.stsCacheUpdater()

	return nil
}

func (rd *RemoteDelivery) Close() error {
	rd.stsCacheUpdateDone <- struct{}{}
	<-rd.stsCacheUpdateDone
	return nil
}

func (rd *RemoteDelivery) Name() string {
	return "remote"
}

func (rd *RemoteDelivery) InstanceName() string {
	return rd.name
}

func (rd *RemoteDelivery) Deliver(ctx module.DeliveryContext, msg io.Reader) error {
	var body io.ReadSeeker
	if seekable, ok := msg.(io.ReadSeeker); ok {
		body = seekable
	} else {
		bodySlice, err := ioutil.ReadAll(msg)
		if err != nil {
			return errors.New("failed to buffer message")
		}
		body = bytes.NewReader(bodySlice)
	}

	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}

	groupedRcpts, err := groupByDomain(ctx.To)
	if err != nil {
		return err
	}

	partialErr := PartialError{
		Errs: make(map[string]error),
	}
	// TODO: look into ways to parallelize this, the main trouble here is body
	// probably create pipe for each server and copy body to each?
	for domain, rcpts := range groupedRcpts {
		perr := rd.deliverForServer(&ctx, domain, rcpts, body)

		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("body seek failed: %v", err)
		}

		if perr != nil {
			rd.Log.Debugf("deliverForServer: %+v (delivery ID = %s)", perr, ctx.DeliveryID)
			for _, successful := range perr.Successful {
				partialErr.Successful = append(partialErr.Successful, successful)
			}
			for _, temporaryFail := range perr.TemporaryFailed {
				partialErr.TemporaryFailed = append(partialErr.TemporaryFailed, temporaryFail)
			}
			for _, failed := range perr.Failed {
				partialErr.Failed = append(partialErr.Failed, failed)
			}
			for k, v := range perr.Errs {
				partialErr.Errs[k] = v
			}
		}

	}

	if len(partialErr.Failed) == 0 && len(partialErr.TemporaryFailed) == 0 {
		return nil
	}
	return partialErr
}

func toPartialError(temporary bool, rcpts []string, err error) *PartialError {
	perr := PartialError{
		Errs: make(map[string]error, len(rcpts)),
	}
	if temporary {
		perr.TemporaryFailed = rcpts
	} else {
		perr.Failed = rcpts
	}
	for _, rcpt := range rcpts {
		perr.Errs[rcpt] = err
	}
	return &perr
}

var ErrNoMXMatchedBySTS = errors.New("remote: no MX record matched MTA-STS policy")

func (rd *RemoteDelivery) deliverForServer(ctx *module.DeliveryContext, domain string, rcpts []string, body io.Reader) *PartialError {
	addrs, err := lookupTargetServers(domain)
	if err != nil {
		return toPartialError(false, rcpts, err)
	}

	stsPolicy, err := rd.mtastsCache.Get(domain)
	if err != nil && err != mtasts.ErrNoPolicy {
		rd.Log.Printf("failed to fetch MTA-STS policy for %s: %v", domain, err)
		// Problems with policy should be treated as temporary ones.
		return toPartialError(true, rcpts, err)
	}
	if stsPolicy != nil && stsPolicy.Mode != mtasts.ModeEnforce {
		// Throw away policy if it is not enforced, we don't do TLSRPT for now.
		// TODO: TLS reporting.
		rd.Log.Debugf("ignoring non-enforced MTA-STS policy for %s", domain)
		stsPolicy = nil
	}

	var cl *smtp.Client
	var lastErr error
	var usedServer string
	for _, addr := range addrs {
		if stsPolicy != nil && !stsPolicy.Match(addr) {
			rd.Log.Printf("ignoring MX record for %s, as it is not matched by MTS-STS policy (%v)", addr, stsPolicy.MX)
			lastErr = ErrNoMXMatchedBySTS
			continue
		}

		cl, err = connectToServer(rd.hostname, addr, rd.requireTLS || stsPolicy != nil)
		if err != nil {
			rd.Log.Debugf("failed to connect to %s: %v", addr, err)
			lastErr = err
			if !isTemporaryErr(err) {
				break
			}
			continue
		}
		usedServer = addr
	}
	if cl == nil {
		return toPartialError(isTemporaryErr(lastErr), rcpts, lastErr)
	}

	rd.Log.Debugln("connected to", usedServer)

	if err := cl.Mail(ctx.From); err != nil {
		rd.Log.Printf("MAIL FROM failed: %v (server = %s, delivery ID = %s)", err, usedServer, ctx.DeliveryID)
		return toPartialError(isTemporaryErr(err), rcpts, err)
	}

	perr := PartialError{Errs: make(map[string]error)}
	for _, rcpt := range rcpts {
		if err := cl.Rcpt(rcpt); err != nil {
			rd.Log.Printf("RCPT TO failed: %v (server = %s, delivery ID = %s)", err, usedServer, ctx.DeliveryID)
			if isTemporaryErr(err) {
				perr.TemporaryFailed = append(perr.TemporaryFailed, rcpt)
			} else {
				perr.Failed = append(perr.Failed, rcpt)
			}
			perr.Errs[rcpt] = err
		}
	}
	bodyWriter, err := cl.Data()
	if err != nil {
		rd.Log.Printf("DATA failed: %v (server = %s, delivery ID = %s)", err, usedServer, ctx.DeliveryID)
		return toPartialError(isTemporaryErr(err), rcpts, err)
	}
	defer bodyWriter.Close()
	if _, err := io.Copy(bodyWriter, body); err != nil {
		rd.Log.Printf("body write failed: %v (server = %s, delivery ID = %s)", err, usedServer, ctx.DeliveryID)
		return toPartialError(isTemporaryErr(err), rcpts, err)
	}

	if err := bodyWriter.Close(); err != nil {
		return toPartialError(isTemporaryErr(err), rcpts, err)
	}

	return nil
}

func (rd *RemoteDelivery) stsCacheUpdater() {
	// Always update cache on start-up since we may have been down for some
	// time.
	rd.Log.Debugln("updating MTA-STS cache...")
	if err := rd.mtastsCache.RefreshCache(); err != nil {
		rd.Log.Printf("MTA-STS cache opdate failed: %v", err)
	}
	rd.Log.Debugln("updating MTA-STS cache... done!")

	for {
		select {
		case <-rd.stsCacheUpdateTick.C:
			rd.Log.Debugln("updating MTA-STS cache...")
			if err := rd.mtastsCache.RefreshCache(); err != nil {
				rd.Log.Printf("MTA-STS cache opdate failed: %v", err)
			}
			rd.Log.Debugln("updating MTA-STS cache... done!")
		case <-rd.stsCacheUpdateDone:
			rd.stsCacheUpdateDone <- struct{}{}
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

func groupByDomain(rcpts []string) (map[string][]string, error) {
	res := make(map[string][]string, len(rcpts))
	for _, rcpt := range rcpts {
		parts := strings.Split(rcpt, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed address: %s", rcpt)
		}

		res[parts[1]] = append(res[parts[1]], rcpt)
	}
	return res, nil
}

func lookupTargetServers(domain string) ([]string, error) {
	records, err := net.LookupMX(domain)
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
	if protoErr, ok := err.(*textproto.Error); ok {
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
	module.Register("remote", NewRemoteDelivery)
}
