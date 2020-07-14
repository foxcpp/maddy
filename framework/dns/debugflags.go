//+build debugflags

package dns

import (
	"flag"
)

func init() {
	flag.StringVar(&overrideServ, "debug.dnsoverride", "system-default", "replace the DNS resolver address")
}
