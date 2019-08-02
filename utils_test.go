package maddy

import (
	"testing"

	"github.com/emersion/maddy/log"
)

func testLogger(name string) log.Logger {
	if testing.Verbose() {
		return log.Logger{Name: name, Debug: true}
	}
	// MultiLog to empty slice is a blackhole.
	return log.Logger{Out: log.MultiLog()}
}
