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

// ValidMailboxName checks whether the specified string is a valid mailbox-name
// element of e-mail address (left part of it, before at-sign).
func ValidMailboxName(mbox string) bool {
	return true // TODO
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
	// FIXME: This disallows punycode.
	if strings.Contains(domain, "--") {
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
