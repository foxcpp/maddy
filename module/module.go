// Package module contians interfaces implemented by maddy modules.
//
// They are moved to separate package to prevent circular dependencies.
//
// Each interface required by maddy for operation is provided by some object
// called "module".  This includes authentication, storage backends, DKIM,
// email filters, etc.  Each module may serve multiple functions. I.e. it can
// be IMAP storage backend, SMTP upstream and authentication provider at the
// same moment.
//
// Each module gets its own unique name (sqlmail for go-sqlmail, proxy for
// proxy module, local for local delivery perhaps, etc). Each module instance
// also gets its own (unique too) name which is used to refer to it in
// configuration.
package module

import (
	"sync"

	"github.com/mholt/caddy/caddyfile"
)

// Module is the interface implemented by all maddy module instances.
//
// It defines basic methods used to identify instances.
type Module interface {
	// Name method reports module name.
	//
	// It is used to reference module in the configuration and in logs.
	Name() string

	// InstanceName method reports unique name of this module instance.
	InstanceName() string

	// Module version. Reported in logs.
	Version() string
}

// NewModule is function that creates new instance of module with specified name.
type NewModule func(name string, cfg map[string][]caddyfile.Token) (Module, error)

// Global WaitGroup instance is used to ensure graceful shutdown of server.
// Whenever module starts goroutine (except when it is short-lived) - it should
// increment WaitGroup counter by 1. Correspondingly, it should call Done when goroutine
// finishes execution.
//
// If module requires special clean-up on shutdown - it should implement io.Closer interface
// for this. Close() method will be called on server shutdown in this case. This method
// should tell all module-created goroutines to stop.
var WaitGroup sync.WaitGroup
