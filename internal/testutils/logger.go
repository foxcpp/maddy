package testutils

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/maddy/framework/log"
)

var (
	debugLog  = flag.Bool("test.debuglog", false, "(maddy) Turn on debug log messages")
	directLog = flag.Bool("test.directlog", false, "(maddy) Log to stderr instead of test log")
)

func Logger(t *testing.T, name string) log.Logger {
	if *directLog {
		return log.Logger{
			Out:   log.WriterOutput(os.Stderr, true),
			Name:  name,
			Debug: *debugLog,
		}
	}

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
