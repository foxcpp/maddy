package address

import (
	"strings"
	"unicode/utf8"

	"github.com/foxcpp/maddy/internal/dns"
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

// ForLookup transforms the local-part of the address into a canonical form
// usable for map lookups or direct comparisons.
//
// If Equal(addr1, addr2) == true, then ForLookup(addr1) == ForLookup(addr2).
//
// On error, case-folded addr is also returned.
func ForLookup(addr string) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return strings.ToLower(addr), err
	}

	if domain != "" {
		domain, err = dns.ForLookup(domain)
		if err != nil {
			return strings.ToLower(addr), err
		}
	}

	mbox = strings.ToLower(norm.NFC.String(mbox))

	if domain == "" {
		return mbox, nil
	}

	return mbox + "@" + domain, nil
}

// CleanDomain returns the address with the domain part converted into its canonical form.
//
// More specifically, converts the domain part of the address to U-labels,
// normalizes it to NFC and then case-folds it.
//
// Original value is also returned on the error.
func CleanDomain(addr string) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return strings.ToLower(addr), err
	}

	uDomain, err := idna.ToUnicode(domain)
	if err != nil {
		return strings.ToLower(addr), err
	}
	uDomain = strings.ToLower(norm.NFC.String(uDomain))

	return mbox + "@" + uDomain, nil
}

// Equal reports whether addr1 and addr2 are considered to be
// case-insensitively equivalent.
//
// The equivalence is defined to be the conjunction of IDN label equivalence
// for the domain part and canonical equivalence* of the local-part converted
// to lower case.
//
// * IDN label equivalence is defined by RFC 5890 Section 2.3.2.4.
// ** Canonical equivalence is defined by UAX #15.
//
// Equivalence for malformed addresses is defined using regular byte-string
// comparison with case-folding applied.
func Equal(addr1, addr2 string) bool {
	// Short circuit. If they are bit-equivalent, then they are also canonically
	// equivalent.
	if addr1 == addr2 {
		return true
	}

	uAddr1, _ := ForLookup(addr1)
	uAddr2, _ := ForLookup(addr2)
	return uAddr1 == uAddr2
}

func IsASCII(s string) bool {
	for _, ch := range s {
		if ch > utf8.RuneSelf {
			return false
		}
	}
	return true
}
