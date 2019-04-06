package log

import (
	"fmt"
	"io"
	"os"
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
	l.log(true, fmt.Sprintf(format+"\n", val...))
}

func (l *Logger) Debugln(val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, fmt.Sprintln(val...))
}

func (l *Logger) Printf(format string, val ...interface{}) {
	l.log(false, fmt.Sprintf(format+"\n", val...))
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

var DefaultLogger = Logger{Out: StderrLog()}

func Debugf(format string, val ...interface{}) { DefaultLogger.Debugf(format, val...) }
func Debugln(val ...interface{})               { DefaultLogger.Debugln(val...) }
func Printf(format string, val ...interface{}) { DefaultLogger.Printf(format, val...) }
func Println(val ...interface{})               { DefaultLogger.Println(val...) }

func StderrLog() FuncLog {
	return WriterLog(os.Stderr)
}

func WriterLog(w io.Writer) FuncLog {
	return func(t time.Time, debug bool, str string) {
		if debug {
			str = "[debug] " + str
		}
		str = t.Format("02.01.06 15:04:05") + " " + str
		io.WriteString(w, str)
	}
}

func MultiLog(outs ...FuncLog) FuncLog {
	return func(t time.Time, debug bool, str string) {
		for _, out := range outs {
			out(t, debug, str)
		}
	}
}
