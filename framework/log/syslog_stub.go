//+build windows plan9

package log

import (
	"errors"
)

// SyslogOutput returns a log.Output implementation that will send
// messages to the system syslog daemon.
//
// Regular messages will be written with INFO priority,
// debug messages will be written with DEBUG priority.
//
// Returned log.Output object is goroutine-safe.
func SyslogOutput() (Output, error) {
	return nil, errors.New("log: syslog output is not supported on windows")
}
