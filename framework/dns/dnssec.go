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

package dns

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/miekg/dns"
)

type TLSA = dns.TLSA

// ClientConfig is the DNS client configuration accepted by NewFromConfig.
type ClientConfig = dns.ClientConfig

// ExtResolver is a convenience wrapper for miekg/dns library that provides
// access to certain low-level functionality (notably, AD flag in responses,
// indicating whether DNSSEC verification was performed by the server).
type ExtResolver struct {
	log *log.Logger

	mu sync.RWMutex

	cl  *dns.Client
	cfg *dns.ClientConfig

	confPath        string
	opts            []Option
	configModTime   time.Time
	lastConfigCheck time.Time
	checking        bool
}

const DefaultConfigPath = "/etc/resolv.conf"

// Option customizes an ExtResolver during construction.
type Option = func(*ExtResolver)

// WithLogger sets the logger used by the resolver.
func WithLogger(logger *log.Logger) Option {
	return func(r *ExtResolver) { r.log = logger }
}

// RCodeError is returned by ExtResolver when the RCODE in response is not
// NOERROR.
type RCodeError struct {
	Name string
	Code int
}

func (err RCodeError) Temporary() bool {
	return err.Code == dns.RcodeServerFailure
}

func (err RCodeError) Error() string {
	switch err.Code {
	case dns.RcodeFormatError:
		return "dns: rcode FORMERR when looking up " + err.Name
	case dns.RcodeServerFailure:
		return "dns: rcode SERVFAIL when looking up " + err.Name
	case dns.RcodeNameError:
		return "dns: rcode NXDOMAIN when looking up " + err.Name
	case dns.RcodeNotImplemented:
		return "dns: rcode NOTIMP when looking up " + err.Name
	case dns.RcodeRefused:
		return "dns: rcode REFUSED when looking up " + err.Name
	}
	return "dns: non-success rcode: " + strconv.Itoa(err.Code) + " when looking up " + err.Name
}

func IsNotFound(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsNotFound
	}

	var rcodeErr RCodeError
	if errors.As(err, &rcodeErr) {
		return rcodeErr.Code == dns.RcodeNameError
	}

	return false
}

func isLoopback(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func (e *ExtResolver) exchange(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	e.ensureActualConfig()

	e.mu.RLock()
	servers, port, cl := e.cfg.Servers, e.cfg.Port, e.cl
	e.mu.RUnlock()

	var resp *dns.Msg
	var lastErr error
	for _, srv := range servers {
		resp, _, lastErr = cl.ExchangeContext(ctx, msg, net.JoinHostPort(srv, port))
		if lastErr != nil {
			continue
		}

		if resp.Rcode != dns.RcodeSuccess {
			lastErr = RCodeError{msg.Question[0].Name, resp.Rcode}
			continue
		}

		// Diregard AD flags from non-local resolvers, likely they are
		// communicated with using an insecure channel and so flags can be
		// tampered with.
		if !isLoopback(srv) {
			resp.AuthenticatedData = false
		}

		break
	}
	return resp, lastErr
}

func (e *ExtResolver) AuthLookupAddr(ctx context.Context, addr string) (ad bool, names []string, err error) {
	revAddr, err := dns.ReverseAddr(addr)
	if err != nil {
		return false, nil, err
	}

	msg := new(dns.Msg)
	msg.SetQuestion(revAddr, dns.TypePTR)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	ad = resp.AuthenticatedData
	names = make([]string, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		ptrRR, ok := rr.(*dns.PTR)
		if !ok {
			continue
		}

		names = append(names, ptrRR.Ptr)
	}
	return
}

func (e *ExtResolver) AuthLookupHost(ctx context.Context, host string) (ad bool, addrs []string, err error) {
	ad, addrParsed, err := e.AuthLookupIPAddr(ctx, host)
	if err != nil {
		return false, nil, err
	}

	addrs = make([]string, 0, len(addrParsed))
	for _, addr := range addrParsed {
		addrs = append(addrs, addr.String())
	}
	return ad, addrs, nil
}

func (e *ExtResolver) AuthLookupMX(ctx context.Context, name string) (ad bool, mxs []*net.MX, err error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.TypeMX)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	ad = resp.AuthenticatedData
	mxs = make([]*net.MX, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		mxRR, ok := rr.(*dns.MX)
		if !ok {
			continue
		}

		mxs = append(mxs, &net.MX{
			Host: mxRR.Mx,
			Pref: mxRR.Preference,
		})
	}
	return
}

func (e *ExtResolver) AuthLookupTXT(ctx context.Context, name string) (ad bool, recs []string, err error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.TypeTXT)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	ad = resp.AuthenticatedData
	recs = make([]string, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		txtRR, ok := rr.(*dns.TXT)
		if !ok {
			continue
		}

		recs = append(recs, strings.Join(txtRR.Txt, ""))
	}
	return
}

// CheckCNAMEAD is a special function for use in DANE lookups. It attempts to determine final
// (canonical) name of the host and also reports whether the whole chain of CNAME's and final zone
// are "secure".
//
// If there are no A or AAAA records for host, rname = "" is returned.
func (e *ExtResolver) CheckCNAMEAD(ctx context.Context, host string) (ad bool, rname string, err error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true
	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, "", err
	}

	for _, r := range resp.Answer {
		switch r := r.(type) {
		case *dns.A:
			rname = r.Hdr.Name
			ad = resp.AuthenticatedData // Use AD flag from response we used to determine rname
		}
	}

	if rname == "" {
		// IPv6-only host? Try to find out rname using AAAA lookup.
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(host), dns.TypeAAAA)
		msg.SetEdns0(4096, false)
		msg.AuthenticatedData = true
		resp, err := e.exchange(ctx, msg)
		if err == nil {
			for _, r := range resp.Answer {
				switch r := r.(type) {
				case *dns.AAAA:
					rname = r.Hdr.Name
					ad = resp.AuthenticatedData
				}
			}
		}
	}

	return ad, rname, nil
}

func (e *ExtResolver) AuthLookupCNAME(ctx context.Context, host string) (ad bool, cname string, err error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeCNAME)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true
	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, "", err
	}

	for _, r := range resp.Answer {
		cnameR, ok := r.(*dns.CNAME)
		if !ok {
			continue
		}
		return resp.AuthenticatedData, cnameR.Target, nil
	}

	return resp.AuthenticatedData, "", nil
}

func (e *ExtResolver) AuthLookupIPAddr(ctx context.Context, host string) (ad bool, addrs []net.IPAddr, err error) {
	// First, query IPv6.
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeAAAA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	aaaaFailed := false
	var (
		v6ad    bool
		v6addrs []net.IPAddr
	)
	if err != nil {
		// Disregard the error for AAAA lookups.
		aaaaFailed = true
		e.log.Error("Network I/O error during AAAA lookup", err, "host", host)
	} else {
		v6addrs = make([]net.IPAddr, 0, len(resp.Answer))
		v6ad = resp.AuthenticatedData
		for _, rr := range resp.Answer {
			aaaaRR, ok := rr.(*dns.AAAA)
			if !ok {
				continue
			}
			v6addrs = append(v6addrs, net.IPAddr{IP: aaaaRR.AAAA})
		}
	}

	// Then repeat query with IPv4.
	msg = new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err = e.exchange(ctx, msg)
	var (
		v4ad    bool
		v4addrs []net.IPAddr
	)
	if err != nil {
		if aaaaFailed {
			return false, nil, err
		}
		// Disregard A lookup error if AAAA succeeded.
		e.log.Error("Network I/O error during A lookup, using AAAA records", err, "host", host)
	} else {
		v4ad = resp.AuthenticatedData
		v4addrs = make([]net.IPAddr, 0, len(resp.Answer))
		for _, rr := range resp.Answer {
			aRR, ok := rr.(*dns.A)
			if !ok {
				continue
			}
			v4addrs = append(v4addrs, net.IPAddr{IP: aRR.A})
		}
	}

	// A little bit of careful handling is required if AD is inconsistent
	// for A and AAAA queries. This unfortunatenly happens in practice. For
	// purposes of DANE handling (A/AAAA check) we disregard AAAA records
	// if they are not authenctiated and return only A records with AD=true.

	addrs = make([]net.IPAddr, 0, len(v4addrs)+len(v6addrs))
	if !v6ad && !v4ad {
		addrs = append(addrs, v6addrs...)
		addrs = append(addrs, v4addrs...)
	} else {
		if v6ad {
			addrs = append(addrs, v6addrs...)
		}
		addrs = append(addrs, v4addrs...)
	}
	return v4ad, addrs, nil
}

func (e *ExtResolver) AuthLookupTLSA(ctx context.Context, service, network, domain string) (ad bool, recs []TLSA, err error) {
	name, err := dns.TLSAName(domain, service, network)
	if err != nil {
		return false, nil, err
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.TypeTLSA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	ad = resp.AuthenticatedData
	recs = make([]dns.TLSA, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		rr, ok := rr.(*dns.TLSA)
		if !ok {
			continue
		}

		recs = append(recs, *rr)
	}
	return
}

// NewFromConfig constructs a resolver with a fixed configuration. It ignores
// system override.
func NewFromConfig(cfg *ClientConfig, opts ...Option) (*ExtResolver, error) {
	// Copy config and fill-in defaults.
	cfgCopy := *cfg
	cfgCopy.Servers = append([]string(nil), cfg.Servers...)
	cfgCopy.Search = append([]string(nil), cfg.Search...)
	if len(cfgCopy.Servers) == 0 {
		cfgCopy.Servers = []string{"127.0.0.1"}
	}

	cl := new(dns.Client)
	cl.Dialer = &net.Dialer{
		Timeout: time.Duration(cfgCopy.Timeout) * time.Second,
	}

	r := &ExtResolver{
		cl:  cl,
		cfg: &cfgCopy,
		log: log.DefaultLogger.Sublogger("dns"),
	}
	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

// NewFromConfigFile constructs a resolver that checks the file for changes.
func NewFromConfigFile(confPath string, opts ...Option) (*ExtResolver, error) {
	info, err := os.Stat(confPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("DNS resolver configuration is not a regular file: %s", confPath)
	}

	cfg, err := dns.ClientConfigFromFile(confPath)
	if err != nil {
		return nil, err
	}

	if overrideServ != "" && overrideServ != "system-default" {
		host, port, err := net.SplitHostPort(overrideServ)
		if err != nil {
			panic(err)
		}

		// Save it as an option so it is reapplied
		// on reload in ensureActualConfig.
		opts = append(opts, func(optR *ExtResolver) {
			optR.cfg.Servers = []string{host}
			optR.cfg.Port = port
		})
	}

	r, err := NewFromConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

	r.confPath = confPath
	r.opts = append(r.opts, opts...)
	r.configModTime = info.ModTime()
	r.lastConfigCheck = time.Now()

	return r, nil
}

// NewExtResolver constructs a resolver using the system configuration.
func NewExtResolver(opts ...Option) (*ExtResolver, error) {
	return NewFromConfigFile(DefaultConfigPath, opts...)
}

// ensureActualConfig checks for changes on demand without holding the mutex during file I/O.
func (e *ExtResolver) ensureActualConfig() {
	// Quick check under read lock.
	e.mu.RLock()
	due := e.confPath != "" && !e.checking && time.Since(e.lastConfigCheck) >= 5*time.Second
	e.mu.RUnlock()
	if !due {
		return
	}

	// Recheck under proper write lock.
	e.mu.Lock()
	if e.checking || time.Since(e.lastConfigCheck) < 5*time.Second {
		e.mu.Unlock()
		return
	}
	e.checking = true
	e.lastConfigCheck = time.Now()
	path, modTime, opts := e.confPath, e.configModTime, e.opts
	e.mu.Unlock()

	info, err := os.Stat(path)
	var updated *ExtResolver
	if err == nil && !info.ModTime().Equal(modTime) {
		var cfg *dns.ClientConfig
		// ClientConfigFromFile's scanner does not report errors encountered
		// during reads, so read the file separately to preserve the old config
		// when it becomes unreadable (for example, if it is replaced by a directory).
		var contents []byte
		if !info.Mode().IsRegular() {
			err = fmt.Errorf("DNS resolver configuration is not a regular file: %s", path)
		} else {
			contents, err = os.ReadFile(path)
		}
		if err == nil {
			// Validate with the same scanner limits as miekg/dns; its parser
			// otherwise silently accepts truncated input after a scanner error.
			scanner := bufio.NewScanner(bytes.NewReader(contents))
			for scanner.Scan() {
			}
			err = scanner.Err()
			if err == nil {
				cfg, err = dns.ClientConfigFromReader(bytes.NewReader(contents))
			}
		}
		if err == nil {
			updated, err = NewFromConfig(cfg, opts...)
		}
	}
	if err != nil {
		e.log.Error("failed to reload DNS resolver configuration", err, "path", path)
	}

	e.mu.Lock()
	if updated != nil {
		e.cfg, e.cl = updated.cfg, updated.cl
		e.configModTime = info.ModTime()
	}
	e.checking = false
	e.mu.Unlock()
}
