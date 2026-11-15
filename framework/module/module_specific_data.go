package module

import (
	"encoding/json"
	"fmt"
	"maps"
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
	modData    map[string]any
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

func (msd *ModSpecificData) Set(m Module, perInstance bool, value any) {
	key := msd.modKey(m, perInstance)
	msd.modDataLck.Lock()
	defer msd.modDataLck.Unlock()
	if msd.modData == nil {
		msd.modData = make(map[string]any)
	}
	msd.modData[key] = value
}

func (msd *ModSpecificData) Get(m Module, perInstance bool) any {
	key := msd.modKey(m, perInstance)
	msd.modDataLck.RLock()
	defer msd.modDataLck.RUnlock()
	return msd.modData[key]
}

// CopyFrom replaces msd's data with a copy of other. Values are copied by
// reference, but the map itself is independent.
func (msd *ModSpecificData) copyFrom(other *ModSpecificData) {
	if msd == other {
		return
	}

	other.modDataLck.RLock()
	var modData map[string]any
	if other.modData != nil {
		modData = maps.Clone(other.modData)
	}
	other.modDataLck.RUnlock()

	msd.modDataLck.Lock()
	msd.modData = modData
	msd.modDataLck.Unlock()
}
