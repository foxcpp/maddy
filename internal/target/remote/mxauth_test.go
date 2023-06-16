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

package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/emersion/go-smtp"
	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/go-mtasts"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/module"
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx2.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
		&localPolicy{minMXLevel: module.MX_MTASTS},
	})
	tgt.tlsConfig = clientCfg
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx4.example.invalid"}, // not mx.example.invalid!
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
		&localPolicy{minMXLevel: module.MX_MTASTS},
	})
	tgt.tlsConfig = clientCfg
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
		&localPolicy{minMXLevel: module.MX_MTASTS},
	})
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			Mode: mtasts.ModeEnforce,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
		&localPolicy{minMXLevel: module.MX_MTASTS},
	})
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
	// At the moment, implementation ensures all MX policy checks are completed
	// before attempting to connect.
	// However, we cannot run complete go-smtp server to check whether it is
	// violated and the connection is actually estabilished since this causes
	// weird race conditions when test completes before go-smtp has the
	// chance to fully initialize itself (Serve is still at the conn.listeners
	// assignment when Close is called).
	//
	// The issue was resolved upstream by introducing locking around internal
	// listeners slice use. Uses of FailOnConn remain since they pretty much do
	// not hurt.
	//
	// https://builds.sr.ht/~emersion/job/147975
	tarpit := testutils.FailOnConn(t, "127.0.0.1:"+smtpPort)
	defer tarpit.Close()

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
	}

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return nil, mtasts.ErrNoPolicy
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
		&localPolicy{minMXLevel: module.MX_MTASTS},
	})
	defer tgt.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
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

	dnsSrv, err := mockdns.NewServerWithLogger(zones, testutils.Logger(t, "mockdns"), false)
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

	dnsSrv, err := mockdns.NewServerWithLogger(zones, testutils.Logger(t, "mockdns"), false)
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

	tgt := testTarget(t, zones, extResolver, []module.MXAuthPolicy{
		&localPolicy{minMXLevel: module.MX_DNSSEC},
	})
	defer tgt.Close()

	_, err = testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Fatal("Expected an error, got none")
	}

	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_REQUIRETLS(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = true
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	testutils.DoTestDeliveryMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_REQUIRETLS_Fail(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = false /* no REQUIRETLS */
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	if _, err := testutils.DoTestDeliveryErrMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	}); err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_REQUIRETLS_Relaxed(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = false /* no REQUIRETLS */
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.relaxedREQUIRETLS = true
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	testutils.DoTestDeliveryMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_REQUIRETLS_Relaxed_NoMXAuth(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = false /* no REQUIRETLS */
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}
		return nil, mtasts.ErrNoPolicy
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.relaxedREQUIRETLS = true
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	if _, err := testutils.DoTestDeliveryErrMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	}); err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_REQUIRETLS_Relaxed_NoTLS(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = false /* no REQUIRETLS */
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.relaxedREQUIRETLS = true
	tgt.tlsConfig = nil
	defer tgt.Close()

	if _, err := testutils.DoTestDeliveryErrMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	}); err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}

func TestRemoteDelivery_REQUIRETLS_Relaxed_TLSFail(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	srv.EnableREQUIRETLS = false /* no REQUIRETLS */
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

	mtastsGet := func(_ context.Context, domain string) (*mtasts.Policy, error) {
		if domain != "example.invalid" {
			return nil, errors.New("Wrong domain in lookup")
		}

		return &mtasts.Policy{
			// Testing policy is enough.
			Mode: mtasts.ModeTesting,
			MX:   []string{"mx.example.invalid"},
		}, nil
	}

	tgt := testTarget(t, zones, nil, []module.MXAuthPolicy{
		testSTSPolicy(t, zones, mtastsGet),
	})
	tgt.relaxedREQUIRETLS = true
	// Cause failure through version incompatibility.
	clientCfg.MaxVersion = tls.VersionTLS13
	clientCfg.MinVersion = tls.VersionTLS13
	srv.TLSConfig.MinVersion = tls.VersionTLS12
	srv.TLSConfig.MaxVersion = tls.VersionTLS12
	tgt.tlsConfig = clientCfg
	defer tgt.Close()

	if _, err := testutils.DoTestDeliveryErrMeta(t, tgt, "test@example.com", []string{"test@example.invalid"}, &module.MsgMetadata{
		OriginalFrom: "test@example.com",
		SMTPOpts: smtp.MailOptions{
			RequireTLS: true,
		},
	}); err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued for server failing authentication")
	}
}
