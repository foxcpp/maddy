package dns

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type TLSA = dns.TLSA

// ExtResolver is a convenience wrapper for miekg/dns library that provides
// access to certain low-level functionality (notably, AD flag in responses,
// indicating whether DNSSEC verification was performed by the server).
type ExtResolver struct {
	cl  *dns.Client
	Cfg *dns.ClientConfig
}

// ErrRCode is returned by ExtResolver when the RCODE in response is not
// NOERROR.
type ErrRCode struct {
	Code int
}

func (err ErrRCode) Error() string {
	switch err.Code {
	case dns.RcodeFormatError:
		return "dns: rcode FORMERR"
	case dns.RcodeServerFailure:
		return "dns: rcode SERVFAIL"
	case dns.RcodeNameError:
		return "dns: rcode NXDOMAIN"
	case dns.RcodeNotImplemented:
		return "dns: rcode NOTIMP"
	case dns.RcodeRefused:
		return "dns: rcode REFUSED"
	}
	return "dns: non-success rcode: " + strconv.Itoa(err.Code)
}

var ErrNotFound = ErrRCode{dns.RcodeNameError}

func (e ExtResolver) exchange(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	var resp *dns.Msg
	var lastErr error
	for _, srv := range e.Cfg.Servers {
		resp, _, lastErr = e.cl.ExchangeContext(ctx, msg, net.JoinHostPort(srv, e.Cfg.Port))
		if lastErr == nil {
			if resp.Rcode != dns.RcodeSuccess {
				return nil, ErrRCode{resp.Rcode}
			}
			break
		}
	}
	return resp, lastErr
}

func (e ExtResolver) AuthLookupAddr(ctx context.Context, addr string) (ad bool, names []string, err error) {
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

func (e ExtResolver) AuthLookupHost(ctx context.Context, host string) (ad bool, addrs []string, err error) {
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

func (e ExtResolver) AuthLookupMX(ctx context.Context, name string) (ad bool, mxs []*net.MX, err error) {
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

func (e ExtResolver) AuthLookupTXT(ctx context.Context, name string) (ad bool, recs []string, err error) {
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

func (e ExtResolver) AuthLookupIPAddr(ctx context.Context, host string) (ad bool, addrs []net.IPAddr, err error) {
	// First, query IPv6.
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeAAAA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err := e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	ad = resp.AuthenticatedData
	addrs = make([]net.IPAddr, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		aaaaRR, ok := rr.(*dns.AAAA)
		if !ok {
			continue
		}
		addrs = append(addrs, net.IPAddr{IP: aaaaRR.AAAA})
	}

	// Then repeat query with IPv4.
	msg = new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
	msg.SetEdns0(4096, false)
	msg.AuthenticatedData = true

	resp, err = e.exchange(ctx, msg)
	if err != nil {
		return false, nil, err
	}

	// Both queries should be authenticated.
	ad = ad && resp.AuthenticatedData

	for _, rr := range resp.Answer {
		aRR, ok := rr.(*dns.A)
		if !ok {
			continue
		}
		addrs = append(addrs, net.IPAddr{IP: aRR.A})
	}

	return ad, addrs, err
}

func (e ExtResolver) AuthLookupTLSA(ctx context.Context, service, network, domain string) (ad bool, recs []TLSA, err error) {
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

func NewExtResolver() (*ExtResolver, error) {
	cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	if overrideServ != "" && overrideServ != "system-default" {
		host, port, err := net.SplitHostPort(overrideServ)
		if err != nil {
			panic(err)
		}
		cfg.Servers = []string{host}
		cfg.Port = port
	}

	cl := new(dns.Client)
	cl.Dialer = &net.Dialer{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}
	return &ExtResolver{
		cl:  cl,
		Cfg: cfg,
	}, nil
}
