package dns

import (
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

// SelectIDNA is a convenience function for encoding to/from Punycode.
//
// If ulabel is true, it returns U-label encoded domain in the Unicode NFC
// form.
// If ulabel is false, it returns A-label encoded domain.
func SelectIDNA(ulabel bool, domain string) (string, error) {
	if ulabel {
		uDomain, err := idna.ToUnicode(domain)
		return norm.NFC.String(uDomain), err
	}
	return idna.ToASCII(domain)
}
