/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Package log implements a minimalistic logging library.
package log

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/foxcpp/maddy/framework/exterrors"
	"go.uber.org/zap"
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
	Fields map[string]interface{}
}

func (l Logger) Zap() *zap.Logger {
	// TODO: Migrate to using zap natively.
	return zap.New(zapLogger{L: l})
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

// Msg writes an event log message in a machine-readable format (currently
// JSON).
//   name: msg\t{"key":"value","key2":"value2"}
//
// Key-value pairs are built from fields slice which should contain key strings
// followed by corresponding values.  That is, for example, []interface{"key",
// "value", "key2", "value2"}.
//
// If value in fields implements LogFormatter, it will be represented by the
// string returned by FormatLog method. Same goes for fmt.Stringer and error
// interfaces.
//
// Additionally, time.Time is written as a string in ISO 8601 format.
// time.Duration follows fmt.Stringer rule above.
func (l Logger) Msg(msg string, fields ...interface{}) {
	m := make(map[string]interface{}, len(fields)/2)
	fieldsToMap(fields, m)
	l.log(false, l.formatMsg(msg, m))
}

// Error writes an event log message in a machine-readable format (currently
// JSON) containing information about the error. If err does have a Fields
// method that returns map[string]interface{}, its result will be added to the
// message.
//   name: msg\t{"key":"value","key2":"value2"}
// Additionally, values from fields will be added to it, as handled by
// Logger.Msg.
//
// In the context of Error method, "msg" typically indicates the top-level
// context in which the error is *handled*. For example, if error leads to
// rejection of SMTP DATA command, msg will probably be "DATA error".
func (l Logger) Error(msg string, err error, fields ...interface{}) {
	if err == nil {
		return
	}

	errFields := exterrors.Fields(err)
	allFields := make(map[string]interface{}, len(fields)+len(errFields)+2)
	for k, v := range errFields {
		allFields[k] = v
	}

	// If there is already a 'reason' field - use it, it probably
	// provides a better explanation than error text itself.
	if allFields["reason"] == nil {
		allFields["reason"] = err.Error()
	}
	fieldsToMap(fields, allFields)

	l.log(false, l.formatMsg(msg, allFields))
}

func (l Logger) DebugMsg(kind string, fields ...interface{}) {
	if !l.Debug {
		return
	}
	m := make(map[string]interface{}, len(fields)/2)
	fieldsToMap(fields, m)
	l.log(true, l.formatMsg(kind, m))
}

func fieldsToMap(fields []interface{}, out map[string]interface{}) {
	var lastKey string
	for i, val := range fields {
		if i%2 == 0 {
			// Key
			key, ok := val.(string)
			if !ok {
				// Misformatted arguments, attempt to provide useful message
				// anyway.
				out[fmt.Sprint("field", i)] = key
				continue
			}
			lastKey = key
		} else {
			// Value
			out[lastKey] = val
		}
	}
}

func (l Logger) formatMsg(msg string, fields map[string]interface{}) string {
	formatted := strings.Builder{}

	formatted.WriteString(msg)
	formatted.WriteRune('\t')

	if len(l.Fields)+len(fields) != 0 {
		if fields == nil {
			fields = make(map[string]interface{})
		}
		for k, v := range l.Fields {
			fields[k] = v
		}
		if err := marshalOrderedJSON(&formatted, fields); err != nil {
			// Fallback to printing the message with minimal processing.
			return fmt.Sprintf("[BROKEN FORMATTING: %v] %v %+v", err, msg, fields)
		}
	}

	return formatted.String()
}

type LogFormatter interface {
	FormatLog() string
}

// Write implements io.Writer, all bytes sent
// to it will be written as a separate log messages.
// No line-buffering is done.
func (l Logger) Write(s []byte) (int, error) {
	l.log(false, strings.TrimRight(string(s), "\n"))
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
