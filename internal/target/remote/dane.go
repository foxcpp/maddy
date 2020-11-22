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

	"github.com/foxcpp/maddy/framework/dns"
	"github.com/foxcpp/maddy/framework/exterrors"
)

// verifyDANE checks whether TLSA records require TLS use and match the
// certificate and name used by the server.
//
// overridePKIX result indicates whether DANE should make server authentication
// succeed even if PKIX/X.509 verification fails. That is, if InsecureSkipVerify
// is used and verifyDANE returns overridePKIX=true, the server certificate
// should trusted.
func verifyDANE(recs []dns.TLSA, serverName string, connState tls.ConnectionState) (overridePKIX bool, err error) {
	tlsErr := &exterrors.SMTPError{
		Code:         550,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
		Message:      "TLS is required but unsupported or failed (enforced by DANE)",
		TargetName:   "remote",
		Misc: map[string]interface{}{
			"remote_server": connState.ServerName,
		},
	}

	// See https://tools.ietf.org/html/rfc6698#appendix-B.2
	// for pseudocode this function is based on.

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
	validRecs := recs[:0]
	for _, rec := range recs {
		switch rec.Usage {
		case 2, 3:
		default:
			continue
		}
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

		validRecs = append(validRecs, rec)
	}

	for _, rec := range validRecs {
		switch rec.Usage {
		case 2: // Trust Anchor Assertion (DANE-TA)
			chains := connState.VerifiedChains
			if len(chains) == 0 { // Happens if InsecureSkipVerify=true
				chains = [][]*x509.Certificate{connState.PeerCertificates}
			}

			for _, chain := range chains {
				for _, cert := range chain {
					if cert.IsCA && rec.Verify(cert) == nil {
						// DANE-TA requires ServerName match, so verify it to
						// override PKIX.
						return chain[0].VerifyHostname(serverName) == nil, nil
					}
				}
			}
		case 3: // Domain issued certificate (DANE-EE)
			if rec.Verify(connState.PeerCertificates[0]) == nil {
				// https://tools.ietf.org/html/rfc7672#section-3.1.1
				// - SAN/CN are not considered so always override.
				// - Expired certificates are fine too.
				return true, nil
			}
		}
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
