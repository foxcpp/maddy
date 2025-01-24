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
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// These certificates are related like this:
//
//	Root A -> Intermediate A -> Leaf A
//	Root B -> LeafB
var (
	rootA = `-----BEGIN CERTIFICATE-----
MIIBMDCB46ADAgECAhRDwag3n5CG90BEO87zEMAPejn6YTAFBgMrZXAwFjEUMBIG
A1UEAxMLVGVzdCBSb290IEEwHhcNMjAxMTI4MjExODA4WhcNMzAxMTI2MjExODA4
WjAWMRQwEgYDVQQDEwtUZXN0IFJvb3QgQTAqMAUGAytlcAMhADXMzcRec5ocluNR
ExnnNT7I5fmcpjf2P4ik5k0DJNbco0MwQTAPBgNVHRMBAf8EBTADAQH/MA8GA1Ud
DwEB/wQFAwMHBAAwHQYDVR0OBBYEFM5b/b1di1vA+YpMZcsF4K7N1LbaMAUGAytl
cANBAAZ0XTxBDjN9VGPqWjXrYqGPUqbjm4JD3PeHUB4YGH+MNTgeVIlU8qCLIXtM
9kmAkCk7+j5G8p0gMjJMNygeuwE=
-----END CERTIFICATE-----`
	intermediateA = `-----BEGIN CERTIFICATE-----
MIIBWjCCAQygAwIBAgIUEOd619/8HC1pWXxaEpQ1vUZOe7wwBQYDK2VwMBYxFDAS
BgNVBAMTC1Rlc3QgUm9vdCBBMB4XDTIwMTEyODIxMTk0M1oXDTMwMTEyNjIxMTk0
M1owHjEcMBoGA1UEAxMTVGVzdCBJbnRlcm1lZGlhdGUgQTAqMAUGAytlcAMhAFgW
aZz5316olEIHn1Q4RTPd2u/EjN2bo+Cn3EmSlFxto2QwYjAPBgNVHRMBAf8EBTAD
AQH/MA8GA1UdDwEB/wQFAwMHBAAwHQYDVR0OBBYEFB0P00Qphygy+KgkI9tjihFD
ELxhMB8GA1UdIwQYMBaAFM5b/b1di1vA+YpMZcsF4K7N1LbaMAUGAytlcANBAJJH
zsS8ahEjdyRCNUlsPalZiKW8N3G0LnwdVKFhVfcCT+RTRcrMP7vjuWsbJyD5e7hu
z2eCI68xreLQlNySdQ0=
-----END CERTIFICATE-----`
	leafA = `-----BEGIN CERTIFICATE-----
MIIBjzCCAUGgAwIBAgIUONvbCs6r9zKFM3IAPRMdrNiJpNgwBQYDK2VwMB4xHDAa
BgNVBAMTE1Rlc3QgSW50ZXJtZWRpYXRlIEEwHhcNMjAxMTI4MjEyMTIyWhcNMzAx
MTI2MjEyMTIyWjAWMRQwEgYDVQQDEwtUZXN0IExlYWYgQTAqMAUGAytlcAMhABIj
W7gwY78RCWHs9eSIdy4x4MXjzdhZwgNSNHHCp5pAo4GYMIGVMAwGA1UdEwEB/wQC
MAAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMBMBUGA1UdEQQOMAyCCm1h
ZGR5LnRlc3QwDwYDVR0PAQH/BAUDAweAADAdBgNVHQ4EFgQU9PFQCnG5fNpNPXUT
8rCuylS6tVwwHwYDVR0jBBgwFoAUHQ/TRCmHKDL4qCQj22OKEUMQvGEwBQYDK2Vw
A0EAGdvHA4VLxpUeUu1Vjom2YX3MukPJG0a3/dB3HiAWWpxMgWfU+Ftie7noaNcI
oUW+M8my46dqN6oXSHU47/QjDg==
-----END CERTIFICATE-----`
	rootB = `-----BEGIN CERTIFICATE-----
MIIBMDCB46ADAgECAhRXD7xuPkipDyxyCtm8pZaxhuulaDAFBgMrZXAwFjEUMBIG
A1UEAxMLVGVzdCBSb290IEIwHhcNMjAxMTI4MjExODMwWhcNMzAxMTI2MjExODMw
WjAWMRQwEgYDVQQDEwtUZXN0IFJvb3QgQjAqMAUGAytlcAMhAPOIGJJh5jK8N/Vc
lLrFpysV+SiZjT1Cmt7hoFtMrlbTo0MwQTAPBgNVHRMBAf8EBTADAQH/MA8GA1Ud
DwEB/wQFAwMHBAAwHQYDVR0OBBYEFOLGYf4mkhKbZPwZKCv952tfz/KDMAUGAytl
cANBAOX2gb6ud8CAvOsCgw6uaRm0+jMDVZfkAkNuCIO6cJ/WYfdvuXYXu3e88SuI
gri++h118PomIzJ5PHAaCYsFPgQ=
-----END CERTIFICATE-----`
	leafB = `-----BEGIN CERTIFICATE-----
MIIBhzCCATmgAwIBAgIUR2bVQ/Cu4j7Td5TdbWd6Q0LEpOgwBQYDK2VwMBYxFDAS
BgNVBAMTC1Rlc3QgUm9vdCBCMB4XDTIwMTEyODIxMjE0M1oXDTMwMTEyNjIxMjE0
M1owFjEUMBIGA1UEAxMLVGVzdCBMZWFmIEIwKjAFBgMrZXADIQBiHCTUxF3UxPIV
M/o5OkTtmUrI7AInOvMa0dchU4iJXqOBmDCBlTAMBgNVHRMBAf8EAjAAMB0GA1Ud
JQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAVBgNVHREEDjAMggptYWRkeS50ZXN0
MA8GA1UdDwEB/wQFAwMHgAAwHQYDVR0OBBYEFPYZPubaAXyr6kXs3khqpMNfdHKK
MB8GA1UdIwQYMBaAFOLGYf4mkhKbZPwZKCv952tfz/KDMAUGAytlcANBABlOwVxE
h7vYmaMYoyOSF1GQiB0ZLsGUjrTNHDnv0+Xp8xG5Td5mGnBi/4Ehq39PdLrj2T7j
3Xy0aiqdDomvwQY=
-----END CERTIFICATE-----`
)

func parsePEMCert(blob string) *x509.Certificate {
	block, _ := pem.Decode([]byte(blob))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(err)
	}
	return cert
}

func singleTlsaRecord(usage, matchType, selector uint8, cert string) dns.TLSA {
	return dns.TLSA{
		Hdr: dns.RR_Header{
			Name:   "maddy.test.",
			Class:  dns.ClassINET,
			Rrtype: dns.TypeTLSA,
			Ttl:    9999,
		},
		Usage:        usage,
		MatchingType: matchType,
		Selector:     selector,
		Certificate:  cert,
	}
}

func keySHA256(blob string) string {
	cert := parsePEMCert(blob)
	hash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(hash[:])
}

func TestVerifyDANE(t *testing.T) {
	verifyDANETime = time.Unix(1606600100, 0)
	test := func(name string, recs []dns.TLSA, connState tls.ConnectionState, expectErr bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()
			_, err := verifyDANE(recs, connState)
			if (err != nil) != expectErr {
				t.Error("err:", err, "expectErr:", expectErr)
			}
		})
	}

	// RFC 7672, Section 2.2:
	// An "insecure" TLSA RRset or DNSSEC-authenticated denial of existence
	// of the TLSA records:
	//    A connection to the MTA SHOULD be made using (pre-DANE)
	// opportunistic TLS;
	//
	// "Insecure" TLSA RRset results in verifyDANE not being called at all,
	// but for the latter (authenticated denial of existence) it is still
	// called and should be tested for.
	//
	// More specific tests for TLSA RRset discovery (including CNAME
	// shenanigans) are in dane_delivery_test.go.
	test("no TLSA, TLS", []dns.TLSA{}, tls.ConnectionState{
		HandshakeComplete: true,
	}, false)
	test("no TLSA, no TLS", []dns.TLSA{}, tls.ConnectionState{
		HandshakeComplete: false,
	}, false)

	// RFC 7272, Section 2.2:
	// A "secure" non-empty TLSA RRset where all the records are unusable:
	//  Any connection to the MTA MUST be made via TLS, but authentication
	//  is not required.
	test("unusable TLSA, TLS", []dns.TLSA{
		singleTlsaRecord(4, 1, 2, "whatever"),
		singleTlsaRecord(4, 5, 2, "whatever"),
		singleTlsaRecord(4, 1, 1, "whatever"),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates:  []*x509.Certificate{parsePEMCert(leafA)},
	}, false)
	test("unusable TLSA, no TLS", []dns.TLSA{
		singleTlsaRecord(4, 1, 2, "whatever"),
	}, tls.ConnectionState{
		HandshakeComplete: false,
	}, true)

	// RFC 7672, Section 2.2:
	// A "secure" TLSA RRset with at least one usable record:  Any
	//  connection to the MTA MUST employ TLS encryption and MUST
	//  authenticate the SMTP server using the techniques discussed in the
	//  rest of this document.
	test("DANE-EE, non-self-signed", []dns.TLSA{
		singleTlsaRecord(3, 1, 1, keySHA256(leafA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates:  []*x509.Certificate{parsePEMCert(leafA)},
	}, false)
	test("DANE-EE, multiple records", []dns.TLSA{
		singleTlsaRecord(3, 1, 1, keySHA256(leafB)),
		singleTlsaRecord(3, 1, 1, keySHA256(leafA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates:  []*x509.Certificate{parsePEMCert(leafA)},
	}, false)
	test("DANE-EE, self-signed", []dns.TLSA{
		singleTlsaRecord(3, 1, 1, keySHA256(rootA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates:  []*x509.Certificate{parsePEMCert(rootA)},
	}, false)
	test("DANE-TA, intermediate TA", []dns.TLSA{
		singleTlsaRecord(2, 1, 1, keySHA256(intermediateA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates: []*x509.Certificate{
			parsePEMCert(leafA),
			parsePEMCert(intermediateA),
			parsePEMCert(rootA),
		},
	}, false)
	test("DANE-TA, intermediate TA, mismatch", []dns.TLSA{
		singleTlsaRecord(2, 1, 1, keySHA256(intermediateA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates: []*x509.Certificate{
			parsePEMCert(leafB),
			parsePEMCert(rootB),
		},
	}, true)
	test("DANE-TA, intermediate TA, multiple records", []dns.TLSA{
		singleTlsaRecord(2, 1, 1, keySHA256(rootB)),
		singleTlsaRecord(2, 1, 1, keySHA256(intermediateA)),
		// Add multiple times to make sure that multiple records matching the
		// same cert do not break anything.
		singleTlsaRecord(2, 1, 1, keySHA256(intermediateA)),
	}, tls.ConnectionState{
		HandshakeComplete: true,
		PeerCertificates: []*x509.Certificate{
			parsePEMCert(leafA),
			parsePEMCert(intermediateA),
			parsePEMCert(rootA),
		},
	}, false)
}
