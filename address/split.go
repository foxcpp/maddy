package address

import (
	"fmt"
	"strings"
)

func Split(addr string) (mailbox, domain string, err error) {
	for i, ch := range addr {
		if ch == '@' {
			return addr[:i], addr[i+1:], nil
		}
	}

	if strings.EqualFold(addr, "postmaster") {
		return addr, "", nil
	}

	return "", "", fmt.Errorf("malformed address")
}
