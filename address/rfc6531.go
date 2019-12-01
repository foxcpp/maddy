package address

import (
	"errors"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrUnicodeMailbox = errors.New("address: can not convert the Unicode local-part to the ACE form")
)

// ToASCII converts the domain part of the email address to the A-label form and
// fails with ErrUnicodeMailbox if the local-part contains non-ASCII characters.
func ToASCII(addr string) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return addr, err
	}

	aDomain, err := idna.ToASCII(domain)
	if err != nil {
		return addr, err
	}

	for _, ch := range mbox {
		if ch > 128 {
			return addr, ErrUnicodeMailbox
		}
	}

	return mbox + "@" + aDomain, nil
}

// ToUnicode converts the domain part of the email address to the U-label form.
func ToUnicode(addr string) (string, error) {
	mbox, domain, err := Split(addr)

	uDomain, err := idna.ToUnicode(domain)
	if err != nil {
		return norm.NFC.String(addr), err
	}

	return mbox + "@" + norm.NFC.String(uDomain), nil
}

// SelectIDNA is a convenience function for conversion of domains in the email
// addresses to/from the Punycode form.
//
// ulabel=true => ToUnicode is used.
// ulabel=false => ToASCII is used.
func SelectIDNA(ulabel bool, addr string) (string, error) {
	if ulabel {
		return ToUnicode(addr)
	}
	return ToASCII(addr)
}
