package target

import (
	"github.com/foxcpp/maddy/internal/log"
	"github.com/foxcpp/maddy/internal/module"
)

func DeliveryLogger(l log.Logger, msgMeta *module.MsgMetadata) log.Logger {
	fields := make(map[string]interface{}, len(l.Fields)+1)
	for k, v := range l.Fields {
		fields[k] = v
	}
	fields["msg_id"] = msgMeta.ID
	l.Fields = fields
	return l
}
