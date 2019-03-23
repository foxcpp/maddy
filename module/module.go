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
	"github.com/emersion/maddy/config"
)

// Module is the interface implemented by all maddy module instances.
//
// It defines basic methods used to identify instances.
//
// Additionally, module can implement io.Closer if it needs to perform clean-up
// on shutdown. If module starts long-lived goroutines - they should be stopped
// *before* Close method returns to ensure graceful shutdown.
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

// FuncNewModule is function that creates new instance of module with specified name.
type FuncNewModule func(name string, globalCfg map[string][]string, cfg config.Node) (Module, error)
