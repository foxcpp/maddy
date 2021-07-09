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
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/foxcpp/maddy/framework/log"
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
	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.IsNotFound
	}
	if rcodeErr, ok := err.(RCodeError); ok {
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

func (e ExtResolver) exchange(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	var resp *dns.Msg
	var lastErr error
	for _, srv := range e.Cfg.Servers {
		resp, _, lastErr = e.cl.ExchangeContext(ctx, msg, net.JoinHostPort(srv, e.Cfg.Port))
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

// CheckCNAMEAD is a special function for use in DANE lookups. It attempts to determine final
// (canonical) name of the host and also reports whether the whole chain of CNAME's and final zone
// are "secure".
//
// If there are no A or AAAA records for host, rname = "" is returned.
func (e ExtResolver) CheckCNAMEAD(ctx context.Context, host string) (ad bool, rname string, err error) {
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
		msg.SetQuestion(dns.Fqdn(host), dns.TypeA)
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

func (e ExtResolver) AuthLookupCNAME(ctx context.Context, host string) (ad bool, cname string, err error) {
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

func (e ExtResolver) AuthLookupIPAddr(ctx context.Context, host string) (ad bool, addrs []net.IPAddr, err error) {
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
		log.DefaultLogger.Error("Network I/O error during AAAA lookup", err, "host", host)
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
		log.DefaultLogger.Error("Network I/O error during A lookup, using AAAA records", err, "host", host)
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

	if len(cfg.Servers) == 0 {
		cfg.Servers = []string{"127.0.0.1"}
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
