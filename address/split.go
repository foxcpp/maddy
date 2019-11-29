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
