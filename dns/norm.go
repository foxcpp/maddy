package dns

import (
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

// ForLookup converts the domain into a canonical form suitable for table
// lookups and other comparisons.
//
// TL;DR Use this instead of strings.ToLower to prepare domain for lookups.
//
// Domains that contain invalid UTF-8 or invalid A-label
// domains are simply converted to local-case using strings.ToLower, but the
// error is also returned.
func ForLookup(domain string) (string, error) {
	uDomain, err := idna.ToUnicode(domain)
	if err != nil {
		return strings.ToLower(domain), err
	}

	// Side note: strings.ToLower does not support full case-folding, so it is
	// important to apply NFC normalization first.
	uDomain = norm.NFC.String(uDomain)
	uDomain = strings.ToLower(uDomain)
	uDomain = strings.TrimSuffix(uDomain, ".")
	return uDomain, nil
}

// Equal reports whether domain1 and domain2 are equivalent as defined by
// IDNA2008 (RFC 5890).
//
// TL;DR Use this instead of strings.EqualFold to compare domains.
//
// Equivalence for malformed A-label domains is defined using regular
// byte-string comparison with case-folding applied.
func Equal(domain1, domain2 string) bool {
	// Short circult. If they are bit-equivalent, then they are also semantically
	// equivalent.
	if domain1 == domain2 {
		return true
	}

	uDomain1, _ := ForLookup(domain1)
	uDomain2, _ := ForLookup(domain2)
	return uDomain1 == uDomain2
}
