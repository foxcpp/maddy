//+build !windows,!plan9

package log

import (
	"fmt"
	"log/syslog"
	"os"
	"time"
)

type syslogOut struct {
	w *syslog.Writer
}

func (s syslogOut) Write(stamp time.Time, debug bool, msg string) {
	var err error
	if debug {
		err = s.w.Debug(msg + "\n")
	} else {
		err = s.w.Info(msg + "\n")
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "!!! Failed to send message to syslog daemon: %v\n", err)
	}
}

func (s syslogOut) Close() error {
	return s.w.Close()
}

// SyslogOutput returns a log.Output implementation that will send
// messages to the system syslog daemon.
//
// Regular messages will be written with INFO priority,
// debug messages will be written with DEBUG priority.
//
// Returned log.Output object is goroutine-safe.
func SyslogOutput() (Output, error) {
	w, err := syslog.New(syslog.LOG_MAIL|syslog.LOG_INFO, "maddy")
	return syslogOut{w}, err
}
