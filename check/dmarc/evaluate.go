package dmarc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dmarc"
	"github.com/foxcpp/maddy/address"
	"github.com/foxcpp/maddy/dns"
	"golang.org/x/net/publicsuffix"
)

func FetchRecord(r dns.Resolver, ctx context.Context, orgDomain, fromDomain string) (*dmarc.Record, error) {
	// 1. Lookup using From Domain.
	txts, err := r.LookupTXT(ctx, "_dmarc."+fromDomain)
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if !ok || !dnsErr.IsNotFound {
			return nil, err
		}
	}
	if len(txts) == 0 {
		// No records or 'no such host', try orgDomain.
		txts, err = r.LookupTXT(ctx, "_dmarc."+orgDomain)
		if err != nil {
			dnsErr, ok := err.(*net.DNSError)
			if !ok || !dnsErr.IsNotFound {
				return nil, err
			}
		}
		// Still nothing? Bail out.
		if len(txts) == 0 {
			return nil, nil
		}
	}

	// Exclude records that are not DMARC policies.
	records := txts[:0]
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=DMARC1") {
			records = append(records, txt)
		}
	}
	// Multiple records => no record.
	if len(records) > 1 || len(records) == 0 {
		return nil, nil
	}

	return dmarc.Parse(records[0])
}

type EvalResult struct {
	Authres     authres.DMARCResult
	SPFResult   authres.SPFResult
	SPFAligned  bool
	DKIMResult  authres.DKIMResult
	DKIMAligned bool
}

func EvaluateAlignment(orgDomain string, record *dmarc.Record, results []authres.Result) EvalResult {
	var (
		spfAligned   = false
		spfResult    = authres.SPFResult{}
		dkimAligned  = false
		dkimResult   = authres.DKIMResult{}
		dkimPresent  = false
		dkimTempFail = false
	)
	for _, res := range results {
		if dkimRes, ok := res.(*authres.DKIMResult); ok {
			dkimPresent = true

			// We want to return DKIM result for a signature provided by the orgDomain,
			// in case there is none - return any (possibly misaligned) for reference.
			if dkimResult.Value == "" {
				dkimResult = *dkimRes
			}
			if isAligned(orgDomain, dkimRes.Domain, record.DKIMAlignment) {
				dkimResult = *dkimRes
				switch dkimRes.Value {
				case authres.ResultPass:
					dkimAligned = true
				case authres.ResultTempError:
					dkimTempFail = true
				}
			}
		}
		if spfRes, ok := res.(*authres.SPFResult); ok {
			spfResult = *spfRes
			var aligned bool
			if spfRes.From == "" {
				aligned = isAligned(orgDomain, spfRes.Helo, record.SPFAlignment)
			} else {
				aligned = isAligned(orgDomain, spfRes.From, record.SPFAlignment)
			}
			if aligned && spfRes.Value == authres.ResultPass {
				spfAligned = true
			}
		}
	}

	res := EvalResult{
		SPFResult:   spfResult,
		SPFAligned:  spfAligned,
		DKIMResult:  dkimResult,
		DKIMAligned: dkimAligned,
	}

	if !dkimPresent || spfResult.Value == "" {
		res.Authres = authres.DMARCResult{
			Value:  authres.ResultNone,
			Reason: "Not enough information (required checks are disabled)",
			From:   orgDomain,
		}
		return res
	}

	if dkimTempFail && !dkimAligned && !spfAligned {
		// We can't be sure whether it is aligned or not. Bail out.
		res.Authres = authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "DKIM authentication temp error",
			From:   orgDomain,
		}
		return res
	}
	if !dkimAligned && spfResult.Value == authres.ResultTempError {
		// We can't be sure whether it is aligned or not. Bail out.
		res.Authres = authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "SPF authentication temp error",
			From:   orgDomain,
		}
		return res
	}

	res.Authres.From = orgDomain
	if dkimAligned || spfAligned {
		res.Authres.Value = authres.ResultPass
	} else {
		res.Authres.Value = authres.ResultFail
		res.Authres.Reason = "No aligned identifiers"
	}
	return res
}

func isAligned(orgDomain, authDomain string, mode dmarc.AlignmentMode) bool {
	switch mode {
	case dmarc.AlignmentStrict:
		return strings.EqualFold(orgDomain, authDomain)
	case dmarc.AlignmentRelaxed:
		return strings.EqualFold(orgDomain, authDomain) ||
			strings.HasSuffix(authDomain, "."+orgDomain)
	}
	// Relaxed alignment by default.
	return strings.EqualFold(orgDomain, authDomain) ||
		strings.HasSuffix(authDomain, "."+orgDomain)
}

func ExtractDomains(hdr textproto.Header) (orgDomain string, fromDomain string, err error) {
	// TODO: Add textproto.Header.Count method.
	var firstFrom string
	for fields := hdr.FieldsByKey("From"); fields.Next(); {
		if firstFrom == "" {
			firstFrom = fields.Value()
		} else {
			return "", "", errors.New("multiple From header fields are not allowed")
		}
	}
	if firstFrom == "" {
		return "", "", errors.New("missing From header field")
	}

	hdrFromList, err := mail.ParseAddressList(firstFrom)
	if err != nil {
		return "", "", fmt.Errorf("malformed From header field: %s", strings.TrimPrefix(err.Error(), "mail: "))
	}
	if len(hdrFromList) > 1 {
		return "", "", errors.New("multiple addresses in From field are not allowed")
	}
	if len(hdrFromList) == 0 {
		return "", "", errors.New("missing address in From field")
	}
	_, domain, err := address.Split(hdrFromList[0].Address)
	if err != nil {
		return "", "", fmt.Errorf("malformed From header field: %w", err)
	}

	orgDomain, err = publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return "", "", err
	}

	return orgDomain, domain, nil
}
