package msgpipeline

import (
	"fmt"

	"github.com/foxcpp/maddy/internal/module"
)

// objectName returns a new that is usable to identify the used external
// component (module or some stub) in debug logs.
func objectName(x interface{}) string {
	mod, ok := x.(module.Module)
	if ok {
		return mod.Name() + ":" + mod.InstanceName()
	}

	_, pipeline := x.(*MsgPipeline)
	if pipeline {
		return "reroute"
	}

	str, ok := x.(fmt.Stringer)
	if ok {
		return str.String()
	}

	return fmt.Sprintf("%T", x)
}
