package hooks

import "sync"

type Event int

const (
	// EventShutdown is triggered when the server process is about to stop.
	EventShutdown Event = iota

	// EventReload is triggered when the server process receives the SIGUSR2
	// signal (on POSIX platforms) and indicates the request to reload the
	// server configuration from persistent storage.
	//
	// Since it is by design problematic to reload the modules configuration,
	// this event only applies to secondary files such as aliases mapping and
	// TLS certificates.
	EventReload

	// EventLogRotate is triggered when the server process receives the SIGUSR1
	// signal (on POSIX platforms) and indicates the request to reopen used log
	// files since they might have rotated.
	EventLogRotate
)

var (
	hooks    = make(map[Event][]func())
	hooksLck sync.Mutex
)

func hooksToRun(eventName Event) []func() {
	hooksLck.Lock()
	defer hooksLck.Unlock()
	hooksEv := hooks[eventName]
	if hooksEv == nil {
		return nil
	}

	// The slice is copied so hooks can be run without holding the lock what
	// might be important since they are likely to do a lot of I/O.
	hooksEvCpy := make([]func(), 0, len(hooksEv))
	hooksEvCpy = append(hooksEvCpy, hooksEv...)

	return hooksEvCpy
}

// RunHooks runs the hooks installed for the specified eventName in the reverse
// order.
func RunHooks(eventName Event) {
	hooks := hooksToRun(eventName)
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i]()
	}
}

// AddHook installs the hook to be executed when certain event occurs.
func AddHook(eventName Event, f func()) {
	hooksLck.Lock()
	defer hooksLck.Unlock()

	hooks[eventName] = append(hooks[eventName], f)
}
