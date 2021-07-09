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

package address

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/foxcpp/maddy/framework/dns"
	"golang.org/x/net/idna"
	"golang.org/x/text/secure/precis"
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
		return addr, err
	}

	uDomain, err := idna.ToUnicode(domain)
	if err != nil {
		return addr, err
	}
	uDomain = strings.ToLower(norm.NFC.String(uDomain))

	if domain == "" {
		return mbox, nil
	}

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

func FQDNDomain(addr string) string {
	if strings.HasSuffix(addr, ".") {
		return addr
	}
	return addr + "."
}

// PRECISFold applies UsernameCaseMapped to the local part and dns.ForLookup
// to domain part of the address.
func PRECISFold(addr string) (string, error) {
	return precisEmail(addr, precis.UsernameCaseMapped)
}

// PRECIS applies UsernameCasePreserved to the local part and dns.ForLookup
// to domain part of the address.
func PRECIS(addr string) (string, error) {
	return precisEmail(addr, precis.UsernameCasePreserved)
}

func precisEmail(addr string, profile *precis.Profile) (string, error) {
	mbox, domain, err := Split(addr)
	if err != nil {
		return "", fmt.Errorf("address: precis: %w", err)
	}

	// PRECISFold is not included in the regular address.ForLookup since it reduces
	// the range of valid addresses to a subset of actually valid values.
	// PRECISFold is a matter of our own local policy, not a general rule for all
	// email addresses.

	// Side note: For used profiles, there is no practical difference between
	// CompareKey and String.
	mbox, err = profile.CompareKey(mbox)
	if err != nil {
		return "", fmt.Errorf("address: precis: %w", err)
	}

	domain, err = dns.ForLookup(domain)
	if err != nil {
		return "", fmt.Errorf("address: precis: %w", err)
	}

	return mbox + "@" + domain, nil
}
