package dns

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/log"
	"github.com/miekg/dns"
)

type TestSrvAction int

const (
	TestSrvTimeout TestSrvAction = iota
	TestSrvServfail
	TestSrvNoAddr
	TestSrvOk
)

func (a TestSrvAction) String() string {
	switch a {
	case TestSrvTimeout:
		return "SrvTimeout"
	case TestSrvServfail:
		return "SrvServfail"
	case TestSrvNoAddr:
		return "SrvNoAddr"
	case TestSrvOk:
		return "SrvOk"
	default:
		panic("wtf action")
	}
}

type IPAddrTestServer struct {
	udpServ    dns.Server
	aAction    TestSrvAction
	aAD        bool
	aaaaAction TestSrvAction
	aaaaAD     bool
}

func (s *IPAddrTestServer) Run() {
	pconn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s.udpServ.PacketConn = pconn
	s.udpServ.Handler = s
	go s.udpServ.ActivateAndServe() //nolint:errcheck
}

func (s *IPAddrTestServer) Close() {
	s.udpServ.PacketConn.Close()
}

func (s *IPAddrTestServer) Addr() *net.UDPAddr {
	return s.udpServ.PacketConn.LocalAddr().(*net.UDPAddr)
}

func (s *IPAddrTestServer) ServeDNS(w dns.ResponseWriter, m *dns.Msg) {
	q := m.Question[0]

	var (
		act TestSrvAction
		ad  bool
	)
	switch q.Qtype {
	case dns.TypeA:
		act = s.aAction
		ad = s.aAD
	case dns.TypeAAAA:
		act = s.aaaaAction
		ad = s.aaaaAD
	default:
		panic("wtf qtype")
	}

	reply := new(dns.Msg)
	reply.SetReply(m)
	reply.RecursionAvailable = true
	reply.AuthenticatedData = ad

	switch act {
	case TestSrvTimeout:
		return // no nobody heard from him since...
	case TestSrvServfail:
		reply.Rcode = dns.RcodeServerFailure
	case TestSrvNoAddr:
	case TestSrvOk:
		switch q.Qtype {
		case dns.TypeA:
			reply.Answer = append(reply.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    9999,
				},
				A: net.ParseIP("127.0.0.1"),
			})
		case dns.TypeAAAA:
			reply.Answer = append(reply.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    9999,
				},
				AAAA: net.ParseIP("::1"),
			})
		}
	}

	if err := w.WriteMsg(reply); err != nil {
		panic(err)
	}
}

func TestExtResolver_AuthLookupIPAddr(t *testing.T) {
	// AuthLookupIPAddr has a rather convoluted logic for combined A/AAAA
	// lookups that return the best-effort result and also has some nuanced in
	// AD flag handling for use in DANE algorithms.

	// Silence log messages about disregarded I/O errors.
	log.DefaultLogger.Out = nil

	test := func(aAct, aaaaAct TestSrvAction, aAD, aaaaAD, ad bool, addrs []net.IP, err bool) {
		t.Helper()
		t.Run(fmt.Sprintln(aAct, aaaaAct, aAD, aaaaAD), func(t *testing.T) {
			t.Helper()

			s := IPAddrTestServer{}
			s.aAction = aAct
			s.aaaaAction = aaaaAct
			s.aAD = aAD
			s.aaaaAD = aaaaAD
			s.Run()
			defer s.Close()
			res := ExtResolver{
				cl: new(dns.Client),
				Cfg: &dns.ClientConfig{
					Servers: []string{"127.0.0.1"},
					Port:    strconv.Itoa(s.Addr().Port),
					Timeout: 1,
				},
			}
			res.cl.Dialer = &net.Dialer{
				Timeout: 500 * time.Millisecond,
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			actualAd, actualAddrs, actualErr := res.AuthLookupIPAddr(ctx, "maddy.test")
			if (actualErr != nil) != err {
				t.Fatal("actualErr:", actualErr, "expectedErr:", err)
			}
			if actualAd != ad {
				t.Error("actualAd:", actualAd, "expectedAd:", ad)
			}
			ipAddrs := make([]net.IPAddr, 0, len(addrs))
			if len(addrs) == 0 {
				ipAddrs = nil // lookup returns nil addrs for error cases
			}
			for _, a := range addrs {
				ipAddrs = append(ipAddrs, net.IPAddr{IP: a, Zone: ""})
			}
			if !reflect.DeepEqual(actualAddrs, ipAddrs) {
				t.Logf("actualAddrs: %#+v", actualAddrs)
				t.Logf("addrs: %#+v", ipAddrs)
				t.Fail()
			}
		})
	}

	test(TestSrvOk, TestSrvOk, true, true, true, []net.IP{net.ParseIP("::1"), net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvOk, true, false, true, []net.IP{net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvOk, false, true, false, []net.IP{net.ParseIP("::1"), net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvOk, false, false, false, []net.IP{net.ParseIP("::1"), net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvTimeout, true, true, true, []net.IP{net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvServfail, true, true, true, []net.IP{net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvOk, TestSrvNoAddr, true, true, true, []net.IP{net.ParseIP("127.0.0.1").To4()}, false)
	test(TestSrvNoAddr, TestSrvOk, true, true, true, []net.IP{net.ParseIP("::1")}, false)
	test(TestSrvServfail, TestSrvServfail, true, true, false, nil, true)

	// actualAd is false, we don't want to risk reporting positive AD result if
	// something is wrong with IPv4 lookup.
	test(TestSrvTimeout, TestSrvOk, true, true, false, []net.IP{net.ParseIP("::1")}, false)
	test(TestSrvServfail, TestSrvOk, true, true, false, []net.IP{net.ParseIP("::1")}, false)
}
