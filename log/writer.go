package log

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type wcOutput struct {
	timestamps bool
	wc         io.WriteCloser
}

func (w wcOutput) Write(stamp time.Time, debug bool, msg string) {
	builder := strings.Builder{}
	if w.timestamps {
		builder.WriteString(stamp.Format("2006-01-02T15:04:05.000 "))
	}
	if debug {
		builder.WriteString("[debug] ")
	}
	builder.WriteString(msg)
	builder.WriteRune('\n')
	if _, err := io.WriteString(w.wc, builder.String()); err != nil {
		fmt.Fprintf(os.Stderr, "!!! Failed to write message to log: %v\n", err)
	}
}

func (w wcOutput) Close() error {
	return w.wc.Close()
}

// WriteCloserOutput returns a log.Output implementation that
// will write formatted messages to the provided io.Writer.
//
// Closing returned log.Output object will close the underlying
// io.WriteCloser.
//
// Written messages will include timestamp formatted with millisecond
// precision and [debug] prefix for debug messages.
// If timestamps argument is false, timestamps will not be added.
//
// Returned log.Output does not provide its own serialization
// so goroutine-safety depends on the io.Writer. Most operating
// systems have atomic (read: thread-safe) implementations for
// stream I/O, so it should be safe to use WriterOutput with os.File.
func WriteCloserOutput(wc io.WriteCloser, timestamps bool) Output {
	return wcOutput{timestamps, wc}
}

type nopCloser struct {
	io.Writer
}

func (nc nopCloser) Close() error {
	return nil
}

// WriterOutput returns a log.Output implementation that
// will write formatted messages to the provided io.Writer.
//
// Closing returned log.Output object will have no effect on the
// underlying io.Writer.
//
// Written messages will include timestamp formatted with millisecond
// precision and [debug] prefix for debug messages.
// If timestamps argument is false, timestamps will not be added.
//
// Returned log.Output does not provide its own serialization
// so goroutine-safety depends on the io.Writer. Most operating
// systems have atomic (read: thread-safe) implementations for
// stream I/O, so it should be safe to use WriterOutput with os.File.
func WriterOutput(w io.Writer, timestamps bool) Output {
	return wcOutput{timestamps, nopCloser{os.Stderr}}
}
