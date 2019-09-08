// Package module contains modules registry and interfaces implemented
// by modules.
//
// Interfaces are placed here to prevent circular dependencies.
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
//
// Module.InstanceName() of the returned module object should return instName.
// aliases slice contains other names that can be used to reference created
// module instance. Here is the example of top-level declarations and the
// corresponding arguments passed to module constructor:
//
// modname { }
// Arguments: "modname", "modname", nil.
//
// modname instname { }
// Arguments: "modname", "instname", nil
//
// modname instname secondname1 secondname2 { }
// Arguments: "modname", "instname", []string{"secondname1", "secondname2"}
//
// Note modules are allowed to attach additional meaning to used names.
// For example, endpoint modules use instance name and aliases as addresses
// to listen on.
type FuncNewModule func(modName, instName string, aliases []string) (Module, error)
