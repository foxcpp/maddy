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
	"errors"
	"strings"
)

// Split splits a email address (as defined by RFC 5321 as a forward-path
// token) into local part (mailbox) and domain.
//
// Note that definition of the forward-path token includes the special
// postmaster address without the domain part. Split will return domain == ""
// in this case.
//
// Split does almost no sanity checks on the input and is intentionally naive.
// If this is a concern, ValidMailbox and ValidDomain should be used on the
// output.
func Split(addr string) (mailbox, domain string, err error) {
	if strings.EqualFold(addr, "postmaster") {
		return addr, "", nil
	}

	indx := strings.LastIndexByte(addr, '@')
	if indx == -1 {
		return "", "", errors.New("address: missing at-sign")
	}
	mailbox = addr[:indx]
	domain = addr[indx+1:]
	if mailbox == "" {
		return "", "", errors.New("address: empty local-part")
	}
	if domain == "" {
		return "", "", errors.New("address: empty domain")
	}
	return
}

// UnquoteMbox undoes escaping and quoting of the local-part.  That is, for
// local-part `"test\" @ test"` it will return `test" @test`.
func UnquoteMbox(mbox string) (string, error) {
	var (
		quoted          bool
		escaped         bool
		terminatedQuote bool
		mailboxB        strings.Builder
	)
	for _, ch := range mbox {
		if terminatedQuote {
			return "", errors.New("address: closing quote should be right before at-sign")
		}

		switch ch {
		case '"':
			if !escaped {
				quoted = !quoted
				if !quoted {
					terminatedQuote = true
				}
				continue
			}
		case '\\':
			if !escaped {
				if !quoted {
					return "", errors.New("address: escapes are allowed only in quoted strings")
				}
				escaped = true
				continue
			}
		case '@':
			if !quoted {
				return "", errors.New("address: extra at-sign in non-quoted local-part")
			}
		}

		escaped = false

		mailboxB.WriteRune(ch)
	}

	if mailboxB.Len() == 0 {
		return "", errors.New("address: empty local part")
	}

	return mailboxB.String(), nil
}

// "specials" from RFC5322 grammar with dot removed (it is defined in grammar separately, for some reason)
var mboxSpecial = map[rune]struct{}{
	'(': {}, ')': {}, '<': {}, '>': {},
	'[': {}, ']': {}, ':': {}, ';': {},
	'@': {}, '\\': {}, ',': {},
	'"': {}, ' ': {},
}

func QuoteMbox(mbox string) string {
	var mailboxEsc strings.Builder
	mailboxEsc.Grow(len(mbox))
	quoted := false
	for _, ch := range mbox {
		if _, ok := mboxSpecial[ch]; ok {
			if ch == '\\' || ch == '"' {
				mailboxEsc.WriteRune('\\')
			}
			mailboxEsc.WriteRune(ch)
			quoted = true
		} else {
			mailboxEsc.WriteRune(ch)
		}
	}
	if quoted {
		return `"` + mailboxEsc.String() + `"`
	}
	return mbox
}
