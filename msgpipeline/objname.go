package msgpipeline

import (
	"fmt"

	"github.com/foxcpp/maddy/module"
)

// objectName returns a new that is usable to identify the used external
// component (module or some stub) in debug logs.
func objectName(x interface{}) string {
	mod, ok := x.(module.Module)
	if ok {
		if mod.InstanceName() == "" {
			return mod.Name()
		}
		return mod.InstanceName()
	}

	_, pipeline := x.(*MsgPipeline)
	if pipeline {
		return "reroute"
	}

	return fmt.Sprintf("%T", x)
}
