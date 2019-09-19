package log

import (
	"time"
)

type Output interface {
	Write(stamp time.Time, debug bool, msg string)
	Close() error
}

type multiOut struct {
	outs []Output
}

func (m multiOut) Write(stamp time.Time, debug bool, msg string) {
	for _, out := range m.outs {
		out.Write(stamp, debug, msg)
	}
}

func (m multiOut) Close() error {
	for _, out := range m.outs {
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func MultiOutput(outputs ...Output) Output {
	return multiOut{outputs}
}

type funcOut struct {
	out   func(time.Time, bool, string)
	close func() error
}

func (f funcOut) Write(stamp time.Time, debug bool, msg string) {
	f.out(stamp, debug, msg)
}

func (f funcOut) Close() error {
	return f.close()
}

func FuncOutput(f func(time.Time, bool, string), close func() error) Output {
	return funcOut{f, close}
}
