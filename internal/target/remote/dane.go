package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/future"
)

func (rd *remoteDelivery) lookupTLSA(ctx context.Context, host string) ([]dns.TLSA, error) {
	ad, recs, err := rd.rt.extResolver.AuthLookupTLSA(ctx, "25", "tcp", host)
	if err != nil {
		return nil, err
	}
	if !ad {
		// Per https://tools.ietf.org/html/rfc7672#section-2.2 we interpret
		// a non-authenticated RRset just like an empty RRset. Side note:
		// "bogus" signatures are expected to be caught by the upstream
		// resolver.
		return nil, nil
	}

	// recs can be empty indicating absence of records.

	return recs, nil
}

func (rd *remoteDelivery) verifyDANE(ctx context.Context, tlsaFut *future.Future, serverName string, connState tls.ConnectionState) (overridePKIX bool, err error) {
	// DANE is disabled.
	if tlsaFut == nil {
		return false, nil
	}

	recsI, err := tlsaFut.GetContext(ctx)
	if err != nil {
		// No records.
		if err == dns.ErrNotFound {
			return false, nil
		}

		// Lookup error here indicates a resolution failure or may also
		// indicate a bogus DNSSEC signature.
		// There is a big problem with differentiating these two.
		//
		// We assume DANE failure in both cases as a safety measure.
		// However, there is a possibility of a temporary error condition,
		// so we mark it as such.
		return false, exterrors.WithTemporary(err, true)
	}
	recs := recsI.([]dns.TLSA)

	return verifyDANE(recs, serverName, connState)
}

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
		case 0, 1, 2, 3:
		default:
			continue
		}
		switch rec.MatchingType {
		case 0, 1, 2:
		default:
			continue
		}

		validRecs = append(validRecs, rec)
	}

	for _, rec := range validRecs {
		switch rec.Usage {
		case 0, 2: // CA constraint (PKIX-TA) and Trust Anchor Assertion (DANE-TA)
			chains := connState.VerifiedChains
			if len(chains) == 0 { // Happens if InsecureSkipVerify=true
				chains = [][]*x509.Certificate{connState.PeerCertificates}
			}

			for _, chain := range chains {
				for _, cert := range chain {
					if cert.IsCA && rec.Verify(cert) == nil {
						// DANE-TA requires ServerName match, so verify it to
						// override PKIX.
						return rec.Usage == 2 &&
							chain[0].VerifyHostname(serverName) == nil, nil
					}
				}
			}
		case 1, 3: // Service certificate constraint (PKIX-EE) and Domain issued certificate (DANE-EE)
			if rec.Verify(connState.PeerCertificates[0]) == nil {
				// https://tools.ietf.org/html/rfc7672#section-3.1.1
				// - SAN/CN are not considered so always override.
				// - Expired certificates are fine too.
				return rec.Usage == 3, nil
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
