// Package log implements minimalistic logging library.
package log

import (
	"fmt"
	"io"
	"log/syslog"
	"os"
	"strings"
	"time"
)

/*
Brutal logging library.
*/

type FuncLog func(t time.Time, debug bool, str string)

type Logger struct {
	Out   FuncLog
	Name  string
	Debug bool
}

func (l *Logger) Debugf(format string, val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, fmt.Sprintf(format, val...))
}

func (l *Logger) Debugln(val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, fmt.Sprintln(val...))
}

func (l *Logger) Printf(format string, val ...interface{}) {
	l.log(false, fmt.Sprintf(format, val...))
}

func (l *Logger) Println(val ...interface{}) {
	l.log(false, fmt.Sprintln(val...))
}

func (l *Logger) Write(s []byte) (int, error) {
	l.log(false, string(s))
	return len(s), nil
}

func (l Logger) DebugWriter() *Logger {
	l.Debug = true
	return &l
}

func (l *Logger) log(debug bool, s string) {
	s = strings.TrimRight(s, "\n\t ")

	if l.Name != "" {
		s = l.Name + ": " + s
	}

	if l.Out != nil {
		l.Out(time.Now(), debug, s)
		return
	}
	if DefaultLogger.Out != nil {
		DefaultLogger.Out(time.Now(), debug, s)
		return
	}

	// Logging is disabled - do nothing.
}

var DefaultLogger = Logger{Out: WriterLog(os.Stderr, false)}

func Debugf(format string, val ...interface{}) { DefaultLogger.Debugf(format, val...) }
func Debugln(val ...interface{})               { DefaultLogger.Debugln(val...) }
func Printf(format string, val ...interface{}) { DefaultLogger.Printf(format, val...) }
func Println(val ...interface{})               { DefaultLogger.Println(val...) }

func WriterLog(w io.Writer, timestamp bool) FuncLog {
	return func(t time.Time, debug bool, str string) {
		builder := strings.Builder{}
		if timestamp {
			builder.WriteString(t.Format("02.01.06 15:04:05.000 "))
		}
		if debug {
			builder.WriteString("[debug] ")
		}
		builder.WriteString(str)
		builder.WriteRune('\n')
		if _, err := io.WriteString(w, builder.String()); err != nil {
			fmt.Fprintf(os.Stderr, "!!! Failed to write message to log: %v\n", err)
		}
	}
}

func Syslog() (FuncLog, error) {
	w, err := syslog.New(syslog.LOG_MAIL|syslog.LOG_INFO, "maddy")
	if err != nil {
		return nil, err
	}

	return func(t time.Time, debug bool, str string) {
		var err error
		if debug {
			err = w.Debug(str + "\n")
		} else {
			err = w.Info(str + "\n")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "!!! Failed to send message to syslog daemon: %v\n", err)
		}
	}, nil
}

func MultiLog(outs ...FuncLog) FuncLog {
	return func(t time.Time, debug bool, str string) {
		for _, out := range outs {
			out(t, debug, str)
		}
	}
}
