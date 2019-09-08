package address

import (
	"fmt"
	"strings"
)

func Split(addr string) (mailbox, domain string, err error) {
	parts := strings.Split(addr, "@")
	switch len(parts) {
	case 1:
		if strings.EqualFold(parts[0], "postmaster") {
			return parts[0], "", nil
		}
		return "", "", fmt.Errorf("malformed address")
	case 2:
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return "", "", fmt.Errorf("malformed address")
		}
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("malformed address")
	}
}
