// Package log implements a minimalistic logging library.
package log

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/foxcpp/maddy/exterrors"
)

// Logger is the structure that writes formatted output to the underlying
// log.Output object.
//
// Logger is stateless and can be copied freely.  However, consider that
// underlying log.Output will not be copied.
//
// Each log message is prefixed with logger name.  Timestamp and debug flag
// formatting is done by log.Output.
//
// No serialization is provided by Logger, its log.Output responsibility to
// ensure goroutine-safety if necessary.
type Logger struct {
	Out   Output
	Name  string
	Debug bool

	// Additional fields that will be added
	// to the Msg output.
	Fields []interface{}
}

func (l Logger) Debugf(format string, val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, l.formatMsg(fmt.Sprintf(format, val...), nil))
}

func (l Logger) Debugln(val ...interface{}) {
	if !l.Debug {
		return
	}
	l.log(true, l.formatMsg(strings.TrimRight(fmt.Sprintln(val...), "\n"), nil))
}

func (l Logger) Printf(format string, val ...interface{}) {
	l.log(false, l.formatMsg(fmt.Sprintf(format, val...), nil))
}

func (l Logger) Println(val ...interface{}) {
	l.log(false, l.formatMsg(strings.TrimRight(fmt.Sprintln(val...), "\n"), nil))
}

// Msg writes an event log message in a loosely defined machine-readable format.
//   name: msg | key=value; key2=value2;
//
// Key-value pairs are built from ctx slice which should
// contain key strings followed by corresponding values.
// That is, for example, []interface{"key", "value", "key2", "value2"}.
//
// Field values are formatted depending on the underlying type as follows:
// - Numbers are added as is.
//    key=5; key2=5.66;
// - Strings are quoted using strconv.Quote
//    key="aaa\nbbb\"ccc"
// - For time.Duration String() is used *without* quoting.
// - time.Time is formatted as 2006-01-02T15:04:05 without quoting.
// - If fmt.Stringer is implemented, strconv.Quote(val.String()) is used
// - If LogFormatter is implemented, FormatLog() is used as is
func (l Logger) Msg(msg string, fields ...interface{}) {
	l.log(false, l.formatMsg(msg, fields))
}

// Error writes an event log message in a loosely defined machine-readable format
// containing information about the error. If err does have a Fields method
// that returns []interface{}, its result will be added to the message.
//   name: kind | key=value; key2=value2;
//
// In the context of Error method, "msg" typically indicates the top-level
// context in which the error is *handled*. For example, if error leads to
// rejection of SMTP DATA command, msg will probably be "DATA error".
//
// See Logger.Msg for how fields are formatted.
func (l Logger) Error(msg string, err error, fields ...interface{}) {
	errFields := exterrors.Fields(err)
	allFields := make([]interface{}, 0, len(fields)+len(errFields)+2)

	errKeys := make([]string, 0, len(errFields))
	for k := range errFields {
		errKeys = append(errKeys, k)
	}
	sort.Strings(errKeys)

	allFields = append(allFields, "reason", err.Error())
	for _, key := range errKeys {
		allFields = append(allFields, key, errFields[key])
	}
	allFields = append(allFields, fields...)

	l.log(false, l.formatMsg(msg, allFields))
}

func (l Logger) DebugMsg(kind string, ctx ...interface{}) {
	l.log(true, l.formatMsg(kind, ctx))
}

func (l Logger) formatMsg(msg string, ctx []interface{}) string {
	formatted := strings.Builder{}

	formatted.WriteString(msg)

	if len(ctx)+len(l.Fields) != 0 {
		formatted.WriteString(" (")
		formatFields(&formatted, ctx, len(l.Fields) != 0)
		formatFields(&formatted, l.Fields, false)
		formatted.WriteString(")")
	}

	return formatted.String()
}

type LogFormatter interface {
	FormatLog() string
}

func formatFields(msg *strings.Builder, ctx []interface{}, lastSemicolon bool) {
	for i, val := range ctx {
		if i%2 == 0 {
			// Key
			msg.WriteString(val.(string))
			msg.WriteString("=")
		} else {
			// Value
			switch val := val.(type) {
			case int:
				msg.WriteString(strconv.FormatInt(int64(val), 10))
			case int8:
				msg.WriteString(strconv.FormatInt(int64(val), 10))
			case int16:
				msg.WriteString(strconv.FormatInt(int64(val), 10))
			case int32:
				msg.WriteString(strconv.FormatInt(int64(val), 10))
			case int64:
				msg.WriteString(strconv.FormatInt(val, 10))
			case uint:
				msg.WriteString(strconv.FormatUint(uint64(val), 10))
			case uint8:
				msg.WriteString(strconv.FormatUint(uint64(val), 10))
			case uint16:
				msg.WriteString(strconv.FormatUint(uint64(val), 10))
			case uint32:
				msg.WriteString(strconv.FormatUint(uint64(val), 10))
			case uint64:
				msg.WriteString(strconv.FormatUint(val, 10))
			case float32:
				msg.WriteString(strconv.FormatFloat(float64(val), 'f', 2, 32))
			case float64:
				msg.WriteString(strconv.FormatFloat(val, 'f', 2, 64))
			case string:
				msg.WriteString(strconv.Quote(val))
			case LogFormatter:
				msg.WriteString(val.FormatLog())
			case time.Time:
				msg.WriteString(val.Format("2006-01-02T15:04:05"))
			case time.Duration:
				msg.WriteString(val.String())
			case fmt.Stringer:
				msg.WriteString(strconv.Quote(val.String()))
			default:
				fmt.Fprintf(msg, `"%#v"`, val)
			}

			if lastSemicolon || i+1 != len(ctx) {
				msg.WriteString("; ")
			}
		}
	}
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
