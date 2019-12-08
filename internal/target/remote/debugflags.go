//+build debugflags

package remote

import "flag"

func init() {
	flag.StringVar(&smtpPort, "debug.smtpport", "25", "SMTP port to use for connections in tests")
}
