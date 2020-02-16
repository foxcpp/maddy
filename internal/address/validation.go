package address

import (
	"strings"
)

/*
Rules for validation are subset of rules listed here:
https://emailregex.com/email-validation-summary/
*/

// Valid checks whether ths string is valid as a email address as defined by
// RFC 5321.
func Valid(addr string) bool {
	if len(addr) > 320 { // RFC 3696 says it's 320, not 255.
		return false
	}

	mbox, domain, err := Split(addr)
	if err != nil {
		return false
	}

	// The only case where this can be true is "postmaster".
	// So allow it.
	if domain == "" {
		return true
	}

	return ValidMailboxName(mbox) && ValidDomain(domain)
}

var validGraphic = map[rune]bool{
	'!': true, '#': true,
	'$': true, '%': true,
	'&': true, '\'': true,
	'*': true, '+': true,
	'-': true, '/': true,
	'=': true, '?': true,
	'^': true, '_': true,
	'`': true, '{': true,
	'|': true, '}': true,
	'~': true,
}

// ValidMailboxName checks whether the specified string is a valid mailbox-name
// element of e-mail address (left part of it, before at-sign).
func ValidMailboxName(mbox string) bool {
	if strings.HasPrefix(mbox, `"`) {
		raw, err := UnquoteMbox(mbox)
		if err != nil {
			return false
		}

		// Inside quotes, any ASCII graphic and space is allowed.
		// Additionally, RFC 6531 extends that to allow any Unicode (UTF-8).
		for _, ch := range raw {
			if ch < ' ' || ch == 0x7F /* DEL */ {
				// ASCII control characters.
				return false
			}
		}
		return true
	}

	// Without quotes, limited set of ASCII graphics is allowed + ASCII
	// alphanumeric characters.
	// RFC 6531 extends that to allow any Unicode (UTF-8).
	for _, ch := range mbox {
		if validGraphic[ch] {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch > 0x7F { // Unicode
			continue
		}

		return false
	}

	return true
}

// ValidDomain checks whether the specified string is a valid DNS domain.
func ValidDomain(domain string) bool {
	if len(domain) > 255 {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	if strings.Contains(domain, "..") {
		return false
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) > 64 {
			return false
		}
	}

	return true
}
