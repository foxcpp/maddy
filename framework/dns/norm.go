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

package dns

import (
	"strings"

	"github.com/miekg/dns"
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

func FQDN(domain string) string {
	return dns.Fqdn(domain)
}

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
