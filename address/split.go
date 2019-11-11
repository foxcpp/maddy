package address

import (
	"errors"
	"strings"
)

// Split splits a email address (as defined by RFC 5321 as a forward-path
// token) into local part (mailbox) and domain.
//
// Note that definition of the forward-path token includes special postmater
// address without the domain part. Split will return domain == "" in this
// case.
//
// Additionally, Split undoes escaping and quoting of local-part.
// That is, for address `"test @ test"@example.org` it will return "test @test"
// and "example.org".
func Split(addr string) (mailbox, domain string, err error) {
	if strings.EqualFold(addr, "postmaster") {
		return addr, "", nil
	}

	var (
		quoted          bool
		escaped         bool
		terminatedQuote bool
		mailboxB        strings.Builder
	)
mboxLoop:
	for i, ch := range addr {
		if terminatedQuote && ch != '@' {
			return "", "", errors.New("address: closing quote should be right before at-sign")
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
					return "", "", errors.New("address: escapes are allowed only in quoted strings")
				}
				escaped = true
				continue
			}
		case '@':
			if !escaped && !quoted {
				domain = addr[i+1:]
				if strings.Contains(domain, "@") {
					return "", "", errors.New("address: multiple at-signs")
				}
				break mboxLoop
			}
		}

		escaped = false

		mailboxB.WriteRune(ch)
	}

	if mailboxB.Len() == 0 {
		return "", "", errors.New("address: empty local part")
	}
	if domain == "" {
		return "", "", errors.New("address: empty domain part")
	}

	return mailboxB.String(), domain, nil
}
