package maddy

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/maddy/log"
)

func testLogger(t *testing.T, name string) log.Logger {
	if testing.Verbose() {
		return log.Logger{
			Out: func(_ time.Time, _ bool, str string) {
				t.Helper()
				t.Log(strings.TrimSuffix(str, "\n"))
			},
			Name:  name,
			Debug: true,
		}
	}

	// MultiLog to empty slice is a blackhole.
	return log.Logger{Out: log.MultiLog()}
}
