package target

import (
	"time"

	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

func DeliveryLogger(l log.Logger, msgMeta *module.MsgMetadata) log.Logger {
	out := l.Out
	if out == nil {
		out = log.DefaultLogger.Out
	}

	return log.Logger{
		Out: func(t time.Time, debug bool, str string) {
			out(t, debug, str+" (msg ID = "+msgMeta.ID+")")
		},
		Name:  l.Name,
		Debug: l.Debug,
	}
}
