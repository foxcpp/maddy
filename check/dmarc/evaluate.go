package dmarc

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/authres"
	"github.com/emersion/go-msgauth/dmarc"
	"github.com/foxcpp/maddy/address"
	"golang.org/x/net/publicsuffix"
)

func FetchRecord(ctx context.Context, hdr textproto.Header) (orgDomain, fromDomain string, record *dmarc.Record, err error) {
	orgDomain, fromDomain, err = extractDomains(hdr)
	if err != nil {
		return "", "", nil, err
	}

	// TODO: Add Lookup(context) method or split methods into net.Lookup and Parse.
	record, err = dmarc.Lookup(orgDomain)
	if err == dmarc.ErrNoPolicy {
		return orgDomain, fromDomain, nil, nil
	}
	return orgDomain, fromDomain, record, err
}

func EvaluateAlignment(orgDomain string, record *dmarc.Record, results []authres.Result) authres.DMARCResult {
	var (
		spfAligned   = false
		spfTempFail  = false
		dkimAligned  = false
		dkimTempFail = false
	)
	for _, res := range results {
		if dkimRes, ok := res.(*authres.DKIMResult); ok {
			if isAligned(orgDomain, dkimRes.Domain, record.DKIMAlignment) {
				switch dkimRes.Value {
				case authres.ResultPass:
					dkimAligned = true
				case authres.ResultTempError:
					dkimTempFail = true
				}
			}
		}
		if spfRes, ok := res.(*authres.SPFResult); ok {
			var aligned bool
			if spfRes.From == "" {
				aligned = isAligned(orgDomain, spfRes.Helo, record.SPFAlignment)
			} else {
				aligned = isAligned(orgDomain, spfRes.From, record.SPFAlignment)
			}
			if aligned {
				switch spfRes.Value {
				case authres.ResultPass:
					spfAligned = true
				case authres.ResultTempError:
					spfTempFail = true
				}
			}
		}
	}

	if dkimTempFail && !dkimAligned && !spfAligned {
		// We can't be sure whether it is aligned or not. Bail out.
		return authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "DKIM authentication temp error",
			From:   orgDomain,
		}
	}
	if !dkimAligned && spfTempFail {
		// We can't be sure whether it is aligned or not. Bail out.
		return authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "SPF authentication temp error",
			From:   orgDomain,
		}
	}

	if dkimAligned || spfAligned {
		return authres.DMARCResult{
			Value: authres.ResultPass,
			From:  orgDomain,
		}
	}
	return authres.DMARCResult{
		Value: authres.ResultFail,
		From:  orgDomain,
	}
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

func extractDomains(hdr textproto.Header) (orgDomain string, fromDomain string, err error) {
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
