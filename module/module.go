// Package module contains interfaces implemented by maddy modules.
//
// They are moved to separate package to prevent circular dependencies.
//
// Each interface required by maddy for operation is provided by some object
// called "module".  This includes authentication, storage backends, DKIM,
// email filters, etc.  Each module may serve multiple functions. I.e. it can
// be IMAP storage backend, SMTP upstream and authentication provider at the
// same moment.
//
// Each module gets its own unique name (sql for go-imap-sql, proxy for
// proxy module, local for local delivery perhaps, etc). Each module instance
// also gets its own (unique too) name which is used to refer to it in
// configuration.
package module

import (
	"github.com/foxcpp/maddy/config"
)

// Module is the interface implemented by all maddy module instances.
//
// It defines basic methods used to identify instances.
//
// Additionally, module can implement io.Closer if it needs to perform clean-up
// on shutdown. If module starts long-lived goroutines - they should be stopped
// *before* Close method returns to ensure graceful shutdown.
type Module interface {
	// Init performs actual initialization of the module.
	//
	// It is not done in FuncNewModule so all module instances are
	// registered at time of initialization, thus initialization does not
	// depends on ordering of configuration blocks and modules can reference
	// each other without any problems.
	//
	// Module can use passed config.Map to read its configuration variables.
	Init(*config.Map) error

	// Name method reports module name.
	//
	// It is used to reference module in the configuration and in logs.
	Name() string

	// InstanceName method reports unique name of this module instance.
	InstanceName() string
}

// FuncNewModule is function that creates new instance of module with specified name.
// Note that this function should not do any form of initialization.
type FuncNewModule func(modName, instanceName string) (Module, error)
