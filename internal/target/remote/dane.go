package remote

import (
	"context"
	"crypto/tls"

	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
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

func (rd *remoteDelivery) verifyDANE(ctx context.Context, conn mxConn, connState tls.ConnectionState) error {
	// DANE is disabled.
	if conn.tlsaFut == nil {
		return nil
	}

	recsI, err := conn.tlsaFut.GetContext(ctx)
	if err != nil {
		// No records.
		if err == dns.ErrNotFound {
			return nil
		}

		// Lookup error here indicates a resolution failure or may also
		// indicate a bogus DNSSEC signature.
		// There is a big problem with differentiating these two.
		//
		// We assume DANE failure in both cases as a safety measure.
		// However, there is a possibility of a temporary error condition,
		// so we mark it as such.
		return exterrors.WithTemporary(err, true)
	}
	recs := recsI.([]dns.TLSA)

	return verifyDANE(recs, connState)
}

func verifyDANE(recs []dns.TLSA, connState tls.ConnectionState) error {
	requireTLS := func() error {
		if !connState.HandshakeComplete {
			return &exterrors.SMTPError{
				Code:         550,
				EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
				Message:      "TLS is required but unsupported or failed (DANE)",
				TargetName:   "remote",
				Misc: map[string]interface{}{
					"remote_server": connState.ServerName,
				},
			}
		}
		return nil
	}

	// See https://tools.ietf.org/html/rfc6698#appendix-B.2
	// for pseudocode this function is based on.

	// See https://tools.ietf.org/html/rfc7672#section-2.2 for requirements of
	// TLS discovery.
	// We assume upstream resolver will generate an error if the DNSSEC
	// signature is bogus so this case is "DNSSEC-authenticated denial of existence".
	if len(recs) == 0 {
		return nil
	}

	// Require TLS even if all records are not usable, per Section 2.2 of RFC 7672.
	if err := requireTLS(); err != nil {
		return err
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
			// For DANE-TA, this should override failing PKIX check, but
			// currently it is not done and I (fox.cpp) do not see a way to
			// implement this. Perhaps we can get away without doing so since
			// no public mail server is going to use invalid certs, right?
			// TODO

			for _, chain := range connState.VerifiedChains {
				for _, cert := range chain {
					if cert.IsCA && rec.Verify(cert) == nil {
						return nil
					}
				}
			}
		case 1, 3: // Service certificate constraint (PKIX-EE) and Domain issued certificate (DANE-EE)
			// Comment for DANE-TA above applies here too for DANE-EE.
			// Also, SMTP DANE profile does not require ServerName match for
			// DANE-EE, but we do not override it too.

			if rec.Verify(connState.PeerCertificates[0]) == nil {
				return nil
			}
		}
	}

	// There are valid records, but none matched.
	return &exterrors.SMTPError{
		Code:         550,
		EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
		Message:      "No matching TLSA records",
		TargetName:   "remote",
		Misc: map[string]interface{}{
			"remote_server": connState.ServerName,
		},
	}
}
