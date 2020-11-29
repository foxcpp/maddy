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
	"crypto/x509"
	"time"

	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
)

// Used to override verification time for DANE-TA tests.
var verifyDANETime time.Time

// verifyDANE checks whether TLSA records require TLS use and match the
// certificate and name used by the server.
//
// overridePKIX result indicates whether DANE should make server authentication
// succeed even if PKIX/X.509 verification fails. That is, if InsecureSkipVerify
// is used and verifyDANE returns overridePKIX=true, the server certificate
// should trusted.
func verifyDANE(recs []dns.TLSA, connState tls.ConnectionState) (overridePKIX bool, err error) {
	tlsErr := &exterrors.SMTPError{
		Code:         550,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
		Message:      "TLS is required but unsupported or failed (enforced by DANE)",
		TargetName:   "remote",
		Misc: map[string]interface{}{
			"remote_server": connState.ServerName,
		},
	}

	// See https://tools.ietf.org/html/rfc7672#section-2.2 for requirements of
	// TLS discovery.
	// We assume upstream resolver will generate an error if the DNSSEC
	// signature is bogus so this case is "DNSSEC-authenticated denial of existence".
	if len(recs) == 0 {
		return false, nil
	}

	// Require TLS even if all records are not usable, per Section 2.2 of RFC 7672.
	if !connState.HandshakeComplete {
		return false, tlsErr
	}

	// Ignore invalid records.
	var (
		eeRecs []dns.TLSA
		taRecs []dns.TLSA
	)
	for _, rec := range recs {
		switch rec.MatchingType {
		case 0, 1, 2:
		default:
			continue
		}
		switch rec.Selector {
		case 0, 1:
		default:
			continue
		}

		switch rec.Usage {
		case 2:
			taRecs = append(taRecs, rec)
		case 3:
			eeRecs = append(eeRecs, rec)
		default:
			continue
		}
	}

	// Authentication is not required if all records are unusable, see
	// RFC 7672 Section 2.1.1.
	if len(eeRecs) == 0 && len(taRecs) == 0 {
		return false, nil
	}

	for _, rec := range eeRecs {
		if rec.Verify(connState.PeerCertificates[0]) == nil {
			// https://tools.ietf.org/html/rfc7672#section-3.1.1
			// - SAN/CN are not considered.
			// - Expired certificates are fine too.
			return true, nil
		}
	}

	// Don't bother building a temporary certificate pool if there are no
	// records to check.
	if len(taRecs) == 0 {
		return true, &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
			Message:      "No matching TLSA records",
			TargetName:   "remote",
			Misc: map[string]interface{}{
				"remote_server": connState.ServerName,
			},
		}
	}

	// Collect certificates presented by the server as possible intermediates.
	// Add all certificates from the chain that match any record to the root
	// pool.
	opts := x509.VerifyOptions{
		DNSName:       connState.ServerName,
		Intermediates: x509.NewCertPool(),
		Roots:         x509.NewCertPool(),
		CurrentTime:   verifyDANETime,
	}
	for _, cert := range connState.PeerCertificates {
		root := false
		for _, rec := range taRecs {
			if cert.IsCA && rec.Verify(cert) == nil {
				opts.Roots.AddCert(cert)
				root = true
			}
		}
		if !root {
			opts.Intermediates.AddCert(cert)
		}
	}

	// ... then run the standard X.509 verification. This will verify that the
	// server certificate chains to any of asserted TA certificates.
	if _, err := connState.PeerCertificates[0].Verify(opts); err == nil {
		return true, nil
	}

	// There are valid records, but none matched.
	return false, &exterrors.SMTPError{
		Code:         550,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
		Message:      "No matching TLSA records",
		TargetName:   "remote",
		Misc: map[string]interface{}{
			"remote_server": connState.ServerName,
		},
	}
}
