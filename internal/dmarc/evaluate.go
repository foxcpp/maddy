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
	"github.com/foxcpp/maddy/framework/address"
	"github.com/foxcpp/maddy/framework/dns"
	"golang.org/x/net/publicsuffix"
)

// FetchRecord looks up the DMARC record relevant for the RFC5322.From domain.
// It returns the record and the domain it was found with (may not be
// equal to the RFC5322.From domain).
func FetchRecord(ctx context.Context, r Resolver, fromDomain string) (policyDomain string, rec *Record, err error) {
	policyDomain = fromDomain

	// 1. Lookup using From Domain.
	txts, err := r.LookupTXT(ctx, dns.FQDN("_dmarc."+fromDomain))
	if err != nil {
		dnsErr, ok := err.(*net.DNSError)
		if !ok || !dnsErr.IsNotFound {
			return "", nil, err
		}
	}
	if len(txts) == 0 {
		// No records or 'no such host', try orgDomain.
		orgDomain, err := publicsuffix.EffectiveTLDPlusOne(fromDomain)
		if err != nil {
			return "", nil, err
		}

		policyDomain = orgDomain

		txts, err = r.LookupTXT(ctx, dns.FQDN("_dmarc."+orgDomain))
		if err != nil {
			dnsErr, ok := err.(*net.DNSError)
			if !ok || !dnsErr.IsNotFound {
				return "", nil, err
			}
		}
		// Still nothing? Bail out.
		if len(txts) == 0 {
			return "", nil, nil
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
		return "", nil, nil
	}

	rec, err = dmarc.Parse(records[0])

	return policyDomain, rec, err
}

type EvalResult struct {
	// The Authentication-Results field generated as a result of the DMARC
	// check.
	Authres authres.DMARCResult

	// The Authentication-Results field for SPF that was considered during
	// alignment check. May be empty.
	SPFResult authres.SPFResult

	// Whether HELO or MAIL FROM match the RFC5322.From domain.
	SPFAligned bool

	// The Authentication-Results field for the DKIM signature that is aligned,
	// if no signatures are aligned - this field contains the result for the
	// first signature. May be empty.
	DKIMResult authres.DKIMResult

	// Whether there is a DKIM signature with the d= field matching the
	// RFC5322.From domain.
	DKIMAligned bool
}

// EvaluateAlignment checks whether identifiers authenticated by SPF and DKIM are in alignment
// with the RFC5322.Domain.
//
// It returns EvalResult which contains the Authres field with the actual check result and
// a bunch of other trace information that can be useful for troubleshooting
// (and also report generation).
func EvaluateAlignment(fromDomain string, record *Record, results []authres.Result) EvalResult {
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
			if isAligned(fromDomain, dkimRes.Domain, record.DKIMAlignment) {
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
				aligned = isAligned(fromDomain, spfRes.Helo, record.SPFAlignment)
			} else {
				aligned = isAligned(fromDomain, spfRes.From, record.SPFAlignment)
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
			From:   fromDomain,
		}
		return res
	}

	if dkimTempFail && !dkimAligned && !spfAligned {
		// We can't be sure whether it is aligned or not. Bail out.
		res.Authres = authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "DKIM authentication temp error",
			From:   fromDomain,
		}
		return res
	}
	if !dkimAligned && spfResult.Value == authres.ResultTempError {
		// We can't be sure whether it is aligned or not. Bail out.
		res.Authres = authres.DMARCResult{
			Value:  authres.ResultTempError,
			Reason: "SPF authentication temp error",
			From:   fromDomain,
		}
		return res
	}

	res.Authres.From = fromDomain
	if dkimAligned || spfAligned {
		res.Authres.Value = authres.ResultPass
	} else {
		res.Authres.Value = authres.ResultFail
		res.Authres.Reason = "No aligned identifiers"
	}
	return res
}

func isAligned(fromDomain, authDomain string, mode AlignmentMode) bool {
	if mode == dmarc.AlignmentStrict {
		return strings.EqualFold(fromDomain, authDomain)
	}

	orgDomainFrom, err := publicsuffix.EffectiveTLDPlusOne(fromDomain)
	if err != nil {
		return false
	}
	authDomainFrom, err := publicsuffix.EffectiveTLDPlusOne(authDomain)
	if err != nil {
		return false
	}

	return strings.EqualFold(orgDomainFrom, authDomainFrom)
}

func ExtractFromDomain(hdr textproto.Header) (string, error) {
	// TODO(GH emersion/go-message#75): Add textproto.Header.Count method.
	var firstFrom string
	for fields := hdr.FieldsByKey("From"); fields.Next(); {
		if firstFrom == "" {
			firstFrom = fields.Value()
		} else {
			return "", errors.New("dmarc: multiple From header fields are not allowed")
		}
	}
	if firstFrom == "" {
		return "", errors.New("dmarc: missing From header field")
	}

	hdrFromList, err := mail.ParseAddressList(firstFrom)
	if err != nil {
		return "", fmt.Errorf("dmarc: malformed From header field: %s", strings.TrimPrefix(err.Error(), "mail: "))
	}
	if len(hdrFromList) > 1 {
		return "", errors.New("dmarc: multiple addresses in From field are not allowed")
	}
	if len(hdrFromList) == 0 {
		return "", errors.New("dmarc: missing address in From field")
	}
	_, domain, err := address.Split(hdrFromList[0].Address)
	if err != nil {
		return "", fmt.Errorf("dmarc: malformed From header field: %w", err)
	}

	return domain, nil
}
