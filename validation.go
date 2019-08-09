package maddy

import (
	"strings"
)

/*
Rules for validation are subset of rules listed here:
https://emailregex.com/email-validation-summary/
*/

func validAddress(addr string) bool {
	if len(addr) > 320 { // RFC 3696 says it's 320, not 255.
		return false
	}
	parts := strings.Split(addr, "@")
	switch len(parts) {
	case 1:
		return strings.EqualFold(addr, "postmaster")
	case 2:
		return validMailboxName(parts[0]) && validDomain(parts[1])
	default:
		return false
	}
}

// validMailboxName checks whether the specified string is a valid mailbox-name
// element of e-mail address (left part of it, before at-sign).
func validMailboxName(mbox string) bool {
	return true // TODO
}

func validDomain(domain string) bool {
	if len(domain) > 255 {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	if strings.Contains(domain, "..") {
		return false
	}
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
