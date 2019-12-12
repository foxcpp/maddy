package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/mtasts"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestRemoteDelivery_AuthMX_Fail(t *testing.T) {
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		Log:           testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		tlsConfig:     clientCfg,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				// Testing policy is enough.
				Mode: mtasts.ModeTesting,
				MX:   []string{"mx.example.invalid"},
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_AuthMX_PreferAuth(t *testing.T) {
	_, be2, srv2 := testutils.SMTPServerSTARTTLS(t, "127.0.0.2:"+smtpPort)
	defer srv2.Close()
	defer testutils.CheckSMTPConnLeak(t, srv2)

	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{
				{Host: "non-auth.example.invalid.", Pref: 5},
				{Host: "mx.example.invalid.", Pref: 10},
			},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"non-auth.example.invalid.": {
			A: []string{"127.0.0.2"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		tlsConfig:     clientCfg,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				// Testing policy is enough.
				Mode: mtasts.ModeTesting,
				MX:   []string{"mx.example.invalid"},
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})

	if be2.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		tlsConfig:     clientCfg,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				Mode: mtasts.ModeEnforce,
				MX:   []string{"mx2.example.invalid"},
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		tlsConfig:     clientCfg,
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				Mode: mtasts.ModeTesting,
				MX:   []string{"mx4.example.invalid"}, // not mx.example.invalid!
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		tlsConfig:     &tls.Config{},
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				Mode: mtasts.ModeEnforce,
				MX:   []string{"mx.example.invalid"},
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:     "remote",
		hostname: "mx.example.com",
		// Client not configured to trust the server cert.
		tlsConfig:     &tls.Config{},
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return &mtasts.Policy{
				Mode: mtasts.ModeEnforce,
				MX:   []string{"mx.example.invalid"},
			}, nil
		},
		Log: testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
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
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		tlsConfig:     clientCfg,
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthMTASTS: {}},
		mtastsGet: func(ctx context.Context, domain string) (*mtasts.Policy, error) {
			if domain != "example.invalid" {
				return nil, errors.New("Wrong domain in lookup")
			}

			return nil, mtasts.ErrIgnorePolicy
		},
		Log: testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be1.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_CommonDomain(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "sub.example.invalid.", Pref: 10}},
		},
		"sub.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthCommonDomain: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_AuthMX_CommonDomain_Fail(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "example.another.invalid.", Pref: 10}},
		},
		"example.another.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthCommonDomain: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_AuthMX_CommonDomain_NotETLDp1(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "invalid.", Pref: 10}},
		},
		"invalid.": {
			A: []string{"127.0.0.1"},
		},
	}
	resolver := &mockdns.Resolver{Zones: zones}

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   nil,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthCommonDomain: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	_, err := testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
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
	resolver := &mockdns.Resolver{Zones: zones}

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

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   extResolver,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthDNSSEC: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@example.invalid"})
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
	resolver := &mockdns.Resolver{Zones: zones}

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

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &mockdns.Resolver{Zones: zones},
		dialer:        resolver.DialContext,
		extResolver:   extResolver,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthDNSSEC: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	_, err = testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_MXAuth_IPLiteral(t *testing.T) {
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
	resolver := mockdns.Resolver{Zones: zones}

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

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &resolver,
		dialer:        resolver.DialContext,
		extResolver:   extResolver,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthDNSSEC: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	testutils.DoTestDelivery(t, &tgt, "test@example.com", []string{"test@[127.0.0.1]"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@[127.0.0.1]"})
}

func TestRemoteDelivery_MXAuth_IPLiteral_Fail(t *testing.T) {
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
	resolver := mockdns.Resolver{Zones: zones}

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

	tgt := Target{
		name:          "remote",
		hostname:      "mx.example.com",
		resolver:      &resolver,
		dialer:        resolver.DialContext,
		extResolver:   extResolver,
		requireMXAuth: true,
		mxAuth:        map[string]struct{}{AuthDNSSEC: {}},
		Log:           testutils.Logger(t, "remote"),
	}

	_, err = testutils.DoTestDeliveryErr(t, &tgt, "test@example.com", []string{"test@[127.0.0.1]"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}
