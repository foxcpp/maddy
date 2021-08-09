package log

import (
	"go.uber.org/zap/zapcore"
)

// TODO: Migrate to using actual zapcore to improve logging performance

type zapLogger struct {
	L Logger
}

func (l zapLogger) Enabled(level zapcore.Level) bool {
	if l.L.Debug {
		return true
	}
	return level > zapcore.DebugLevel
}

func (l zapLogger) With(fields []zapcore.Field) zapcore.Core {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	newF := make(map[string]interface{}, len(l.L.Fields)+len(enc.Fields))
	for k, v := range l.L.Fields {
		newF[k] = v
	}
	for k, v := range enc.Fields {
		newF[k] = v
	}
	l.L.Fields = newF
	return l
}

func (l zapLogger) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if l.Enabled(entry.Level) {
		return ce.AddCore(entry, l)
	}
	return ce
}

func (l zapLogger) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	if entry.LoggerName != "" {
		l.L.Name += "/" + entry.LoggerName
	}
	l.L.log(entry.Level == zapcore.DebugLevel, l.L.formatMsg(entry.Message, enc.Fields))
	return nil
}

func (zapLogger) Sync() error {
	return nil
}
