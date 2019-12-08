package updatepipe

type BackendMode int

const (
	// ModeReplicate configures backend to both send and receive updates over
	// the pipe.
	ModeReplicate BackendMode = iota

	// ModePush configures backend to send updates over the pipe only.
	//
	// If EnableUpdatePipe(ModePush) is called for backend, its Updates()
	// channel will never receive any updates.
	ModePush BackendMode = iota
)

// The Backend interface is implemented by storage backends that support both
// updates serialization using the internal updatepipe.P implementation.
// To activate this implementation, EnableUpdatePipe should be called.
type Backend interface {
	// EnableUpdatePipe enables the internal update pipe implementation.
	// The mode argument selects the pipe behavior. EnableUpdatePipe must be
	// called before the first call to the Updates() method.
	//
	// This method is idempotent. All calls after a successful one do nothing.
	EnableUpdatePipe(mode BackendMode) error
}
