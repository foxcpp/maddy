package remote

import (
	"crypto/tls"
	"net"
	"strconv"
	"testing"

	"github.com/foxcpp/go-mockdns"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/testutils"
	miekgdns "github.com/miekg/dns"
)

func targetWithExtResolver(t *testing.T, zones map[string]mockdns.Zone) (*mockdns.Server, *Target) {
	dnsSrv, err := mockdns.NewServer(zones)
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

	tgt := testTarget(t, zones, extResolver, []Policy{
		testDANEPolicy(t, extResolver),
	})
	dnsSrv.Log = tgt.Log
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
			AD: true,
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()
	tgt.tlsConfig = clientCfg

	testutils.DoTestDelivery(t, tgt, "test@example.com", []string{"test@example.invalid"})
	be.CheckMsg(t, 0, "test@example.com", []string{"test@example.invalid"})
}

func TestRemoteDelivery_DANE_NonADIgnore(t *testing.T) {
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
		"_25._tcp.mx.example.invalid.": {
			Misc: tlsaRecord(
				"_25._tcp.mx.example.invalid.",
				3, 1, 1, "a9b5cb4d02f996f6385debe9a8952f1af1f4aec7eae0f37c2cd6d0d8ee8391cf"),
		},
	}

	dnsSrv, tgt := targetWithExtResolver(t, zones)
	defer dnsSrv.Close()

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
			A: []string{"127.0.0.1"},
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
			A: []string{"127.0.0.1"},
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
			A: []string{"127.0.0.1"},
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
			A: []string{"127.0.0.1"},
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
			A: []string{"127.0.0.1"},
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
		MaxVersion: tls.VersionTLS12,
		MinVersion: tls.VersionTLS12,
	}
	srv.TLSConfig.MinVersion = tls.VersionTLS11
	srv.TLSConfig.MaxVersion = tls.VersionTLS11

	// DANE should prevent the fallback to plaintext.
	_, err := testutils.DoTestDeliveryErr(t, tgt, "test@example.com", []string{"test@example.invalid"})
	if err == nil {
		t.Error("Expected an error, got none")
	}
	if be.MailFromCounter != 0 {
		t.Fatal("MAIL FROM issued but should not")
	}
}

// TODO(GH #90): Test other matching types, etc
