//+build debugflags

package maddy

import (
	_ "net/http/pprof"
)

func init() {
	enableDebugFlags = true
}
