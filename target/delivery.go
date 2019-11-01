package target

import (
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
)

func DeliveryLogger(l log.Logger, msgMeta *module.MsgMetadata) log.Logger {
	eventCtx := make([]interface{}, 0, len(l.Fields)+2)
	copy(eventCtx, l.Fields)
	eventCtx = append(eventCtx, "msg_id", msgMeta.ID)

	l.Fields = eventCtx
	return l
}
