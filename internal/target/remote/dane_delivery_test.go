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
	"crypto/tls"
	"net"
	"strconv"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
	miekgdns "github.com/miekg/dns"
)

func targetWithExtResolver(t *testing.T, zones map[string]mockdns.Zone) (*mockdns.Server, *Target) {
	dnsSrv, err := mockdns.NewServerWithLogger(zones, testutils.Logger(t, "mockdns"), false)
	if err != nil {
		t.Fatal(err)
	}

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
		testDANEPolicy(t, extResolver),
	})
	return dnsSrv, tgt
}

func tlsaRecord(name string, usage, matchType, selector uint8, cert string) map[miekgdns.Type][]miekgdns.RR {
	return map[miekgdns.Type][]miekgdns.RR{
		miekgdns.Type(miekgdns.TypeTLSA): {
			&miekgdns.TLSA{
				Hdr: miekgdns.RR_Header{
					Name:   name,
					Class:  miekgdns.ClassINET,
					Rrtype: miekgdns.TypeTLSA,
					Ttl:    9999,
				},
				Usage:        usage,
				MatchingType: matchType,
				Selector:     selector,
				Certificate:  cert,
			},
		},
	}
}

func TestRemoteDelivery_DANE_Ok(t *testing.T) {
	_, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Non-CNAME" case.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.policies = append(tgt.policies,
		&localPolicy{
			minTLSLevel: module.TLSAuthenticated, // Established via DANE instead of PKIX.
		},
	)

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_CNAMEd_1(t *testing.T) {
	_, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Secure CNAME" case - TLSA at CNAME matches.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD:    true,
			CNAME: "mx.cname.invalid.",
		},
		"mx.cname.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"_25._tcp.mx.cname.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.cname.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.policies = append(tgt.policies,
		&localPolicy{
			minTLSLevel: module.TLSAuthenticated, // Established via DANE instead of PKIX.
		},
	)

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_CNAMEd_2(t *testing.T) {
	_, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Secure CNAME" case - TLSA at initial name matches.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD:    true,
			CNAME: "mx.cname.invalid.",
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.cname.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
		"mx.cname.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.policies = append(tgt.policies,
		&localPolicy{
			minTLSLevel: module.TLSAuthenticated, // Established via DANE instead of PKIX.
		},
	)

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_InsecureCNAMEDest(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Insecure CNAME" case - initial name is secure.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD:    true,
			CNAME: "mx.cname.invalid.",
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			// This is the record that activates DANE but does not match the cert
			// => delivery is failed.
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cb"),
		},
		"_25._tcp.mx.cname.invalid.": {
			AD: false,
			// This is the record that matches the cert and would make delivery succeed
			// but it should not be considered since AD=false.
			Misc: tlsaRecord(
				"_25._tcp.mx.cname.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}

func TestRemoteDelivery_DANE_NonAD_TLSA_Ignore(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Non-CNAME" case - initial name is insecure.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cb"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_NonADIgnore_CNAME(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	// RFC 7672, Section 2.2.2. "Insecure CNAME" case - initial name is insecure.
	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			CNAME: "mx.cname.invalid.",
		},
		"mx.cname.invalid.": {
			A: []string{"127.0.0.1"},
		},
		"_25._tcp.mx.cname.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cb"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_SkipAUnauth(t *testing.T) {
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
		"_25._tcp.mx.example.invalid.": {
			AD: false,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "invalid hex will cause serialization error and no response will be sent"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_Mismatch(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "ffb5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}

func TestRemoteDelivery_DANE_NoRecord(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_LookupErr(t *testing.T) {
	clientCfg, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			Err: &net.DNSError{},
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}

func TestRemoteDelivery_DANE_NoTLS(t *testing.T) {
	be, srv := testutils.SMTPServer(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}
	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()

	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}

func TestRemoteDelivery_DANE_TLSError(t *testing.T) {
	_, be, srv := testutils.SMTPServerSTARTTLS(t, "127.0.0.1:"+smtpPort)
	defer srv.Close()
	defer testutils.CheckSMTPConnLeak(t, srv)

	zones := map[string]mockdns.Zone{
		"example.invalid.": {
			AD: true,
			MX: []net.MX{{Host: "mx.example.invalid.", Pref: 10}},
		},
		"mx.example.invalid.": {
			AD: true,
			A:  []string{"127.0.0.1"},
		},
		"_25._tcp.mx.example.invalid.": {
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}
	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()

	// Cause failure through version incompatibility.
	tgt.tlsConfig = &tls.Config{
		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,
	}
	srv.TLSConfig.MinVersion = tls.VersionTLS12
	srv.TLSConfig.MaxVersion = tls.VersionTLS12

	// DANE should prevent the fallback to plaintext.
	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}
