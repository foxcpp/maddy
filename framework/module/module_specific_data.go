package module

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ModSpecificData is a container that allows modules to attach
// additional context data to framework objects such as SMTP connections
// without conflicting with each other and ensuring each module
// gets its own namespace.
//
// It must not be used to store stateful objects that may need
// a specific cleanup routine as ModSpecificData does not provide
// any lifetime management.
//
// Stored data must be serializable to JSON for state persistence
// e.g. when message is stored in a on-disk queue.
type ModSpecificData struct {
	modDataLck sync.RWMutex
	modData    map[string]interface{}
}

func (msd *ModSpecificData) modKey(m Module, perInstance bool) string {
	if !perInstance {
		return m.Name()
	}
	instName := m.InstanceName()
	if instName == "" {
		instName = fmt.Sprintf("%x", m)
	}
	return m.Name() + "/" + instName
}

func (msd *ModSpecificData) MarshalJSON() ([]byte, error) {
	msd.modDataLck.RLock()
	defer msd.modDataLck.RUnlock()
	return json.Marshal(msd.modData)
}

func (msd *ModSpecificData) UnmarshalJSON(b []byte) error {
	msd.modDataLck.Lock()
	defer msd.modDataLck.Unlock()
	return json.Unmarshal(b, &msd.modData)
}

func (msd *ModSpecificData) Set(m Module, perInstance bool, value interface{}) {
	key := msd.modKey(m, perInstance)
	msd.modDataLck.Lock()
	defer msd.modDataLck.Unlock()
	if msd.modData == nil {
		msd.modData = make(map[string]interface{})
	}
	msd.modData[key] = value
}

func (msd *ModSpecificData) Get(m Module, perInstance bool) interface{} {
	key := msd.modKey(m, perInstance)
	msd.modDataLck.RLock()
	defer msd.modDataLck.RUnlock()
	return msd.modData[key]
}
