package exterrors

import (
	"net"
)

func UnwrapDNSErr(err error) (reason string, misc map[string]interface{}) {
	dnsErr, ok := err.(*net.DNSError)
	if !ok {
		// Return non-nil in case the user will try to 'extend' it with its own
		// values.
		return "", map[string]interface{}{}
	}

	// Nor server name, nor DNS name are usually useful, so exclude them.
	return dnsErr.Err, map[string]interface{}{}
}
