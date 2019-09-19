// Package log implements a minimalistic logging library.
package log

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// Logger is the structure that writes formatted output
// to the underlying log.Output object.
//
// Logger is stateless and can be copied freely.
// However, consider that underlying log.Output will not
// be copied.
//
// Each log message is prefixed with logger name.
// Timestamp and debug flag formatting is done by log.Output.
//
// No serialization is provided by Logger, its log.Output
// responsibility to ensure goroutine-safety if necessary.
type Logger struct {
	Out   Output
	Name  string
	Debug bool
}

func (l Logger) Debugf(format string, val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, fmt.Sprintf(format, val...))
}

func (l Logger) Debugln(val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, fmt.Sprintln(val...))
}

func (l Logger) Printf(format string, val ...interface{}) {
	l.log(false, fmt.Sprintf(format, val...))
}

func (l Logger) Println(val ...interface{}) {
	l.log(false, fmt.Sprintln(val...))
}

// Write implements io.Writer, all bytes sent
// to it will be written as a separate log messages.
// No line-buffering is done.
func (l Logger) Write(s []byte) (int, error) {
	l.log(false, string(s))
	return len(s), nil
}

// DebugWriter returns a writer that will act like Logger.Write
// but will use debug flag on messages. If Logger.Debug is false,
// Write method of returned object will be no-op.
func (l Logger) DebugWriter() io.Writer {
	if !l.Debug {
		return ioutil.Discard
	}
	l.Debug = true
	return &l
}

func (l Logger) log(debug bool, s string) {
	s = strings.TrimRight(s, "\n\t ")

	if l.Name != "" {
		s = l.Name + ": " + s
	}

	if l.Out != nil {
		l.Out.Write(time.Now(), debug, s)
		return
	}
	if DefaultLogger.Out != nil {
		DefaultLogger.Out.Write(time.Now(), debug, s)
		return
	}

	// Logging is disabled - do nothing.
}

// DefaultLogger is the global Logger object that is used by
// package-level logging functions.
//
// As with all other Loggers, it is not gorountine-safe on its own,
// however underlying log.Output may provide necessary serialization.
var DefaultLogger = Logger{Out: WriterOutput(os.Stderr, false)}

func Debugf(format string, val ...interface{}) { DefaultLogger.Debugf(format, val...) }
func Debugln(val ...interface{})               { DefaultLogger.Debugln(val...) }
func Printf(format string, val ...interface{}) { DefaultLogger.Printf(format, val...) }
func Println(val ...interface{})               { DefaultLogger.Println(val...) }
