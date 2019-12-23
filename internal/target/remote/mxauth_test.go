package remote

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/go-mtasts"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestRemoteDelivery_AuthMX_MTASTS(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_MTASTS_SkipNonMatching(t *testing.T) {
	_, be1, srv1 := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)

	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.2:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{
				{Host: "mx2.example.invalid.", Pref: 5},
				{Host: "mx1.example.invalid.", Pref: 10},
			},
		},
		"mx1.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"mx2.example.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx2.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	tgt.localPolicy.minMXLevel = MX_MTASTS
	defer tgt.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_MTASTS_Fail(t *testing.T) {
	clientCfg, be1, srv1 := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx4.example.invalid"}, // not mx.example.invalid!
		}, nil
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	tgt.localPolicy.minMXLevel = MX_MTASTS
	defer tgt.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_MTASTS_NoTLS(t *testing.T) {
	be1, srv1 := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.localPolicy.minMXLevel = MX_MTASTS
	defer tgt.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_MTASTS_RequirePKIX(t *testing.T) {
	_, be1, srv1 := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.localPolicy.minMXLevel = MX_MTASTS
	defer tgt.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_MTASTS_NoPolicy(t *testing.T) {
	clientCfg, be1, srv1 := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv1.Close()
	defer testutils.CheckSMTPConnLeak(t, srv1)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(ctx context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return nil, mtasts.ErrNoPolicy
	}

	tgt := testTarget(t, zones, nil, []Policy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	tgt.localPolicy.minMXLevel = MX_MTASTS
	defer tgt.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_DNSSEC(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			AD: true,
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	dnsSrv, err := mockdns.NewServer(zones)
	if err != nil {
		t.Fatal(err)
	}
	defer dnsSrv.Close()

	dialer := net.Dialer{}
	dialer.Resolver = &net.Resolver{}
	dnsSrv.PatchNet(dialer.Resolver)
	addr := dnsSrv.LocalAddr().(*net.UDPAddr)

	extResolver, err := dns.NewExtResolver()
	if err != nil {
		t.Fatal(err)
	}
	extResolver.Cfg.Servers = []string{addr.IP.String()}
	extResolver.Cfg.Port = strconv.Itoa(addr.Port)

	tgt := testTarget(t, zones, extResolver, nil)
	defer tgt.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_AuthMX_DNSSEC_Fail(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	dnsSrv, err := mockdns.NewServer(zones)
	if err != nil {
		t.Fatal(err)
	}
	defer dnsSrv.Close()

	dialer := net.Dialer{}
	dialer.Resolver = &net.Resolver{}
	dnsSrv.PatchNet(dialer.Resolver)
	addr := dnsSrv.LocalAddr().(*net.UDPAddr)

	extResolver, err := dns.NewExtResolver()
	if err != nil {
		t.Fatal(err)
	}
	extResolver.Cfg.Servers = []string{addr.IP.String()}
	extResolver.Cfg.Port = strconv.Itoa(addr.Port)

	tgt := testTarget(t, zones, extResolver, nil)
	tgt.localPolicy.minMXLevel = MX_DNSSEC
	defer tgt.Close()

	_, err = testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_MXAuth_IPLiteral(t *testing.T) {
	t.Skip("Support disabled")

	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"1.0.0.127.in-addr.arpa.": {
			AD:  true,
			PTR: []string{"mx.example.invalid."},
		},
	}

	dnsSrv, err := mockdns.NewServer(zones)
	if err != nil {
		t.Fatal(err)
	}
	defer dnsSrv.Close()

	dialer := net.Dialer{}
	dialer.Resolver = &net.Resolver{}
	dnsSrv.PatchNet(dialer.Resolver)
	addr := dnsSrv.LocalAddr().(*net.UDPAddr)

	extResolver, err := dns.NewExtResolver()
	if err != nil {
		t.Fatal(err)
	}
	extResolver.Cfg.Servers = []string{addr.IP.String()}
	extResolver.Cfg.Port = strconv.Itoa(addr.Port)

	tgt := testTarget(t, zones, extResolver, nil)
	tgt.localPolicy.minMXLevel = MX_DNSSEC
	defer tgt.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@[127.0.0.1]"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@[127.0.0.1]"})
}

func TestRemoteDelivery_MXAuth_IPLiteral_Fail(t *testing.T) {
	t.Skip("Support disabled")

	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"1.0.0.127.in-addr.arpa.": {
			PTR: []string{"mx.example.invalid."},
		},
	}

	dnsSrv, err := mockdns.NewServer(zones)
	if err != nil {
		t.Fatal(err)
	}
	defer dnsSrv.Close()

	dialer := net.Dialer{}
	dialer.Resolver = &net.Resolver{}
	dnsSrv.PatchNet(dialer.Resolver)
	addr := dnsSrv.LocalAddr().(*net.UDPAddr)

	extResolver, err := dns.NewExtResolver()
	if err != nil {
		t.Fatal(err)
	}
	extResolver.Cfg.Servers = []string{addr.IP.String()}
	extResolver.Cfg.Port = strconv.Itoa(addr.Port)

	tgt := testTarget(t, zones, extResolver, nil)
	tgt.localPolicy.minMXLevel = MX_DNSSEC

	_, err = testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@[127.0.0.1]"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}
