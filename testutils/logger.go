package testutils

import (
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/maddy/log"
)

var (
	debugLog = flag.Bool("test.debuglog", false, "Turn on debug log messages")
)

func Logger(t *testing.T, name string) log.Logger {
	return log.Logger{
		Out: log.FuncOutput(func(_ time.Time, debug bool, str string) {
			t.Helper()
			str = strings.TrimSuffix(str, "\n")
			if debug {
				str = "[debug] " + str
			}
			t.Log(str)
		}, func() error {
			return nil
		}),
		Name:  name,
		Debug: *debugLog,
	}
}
