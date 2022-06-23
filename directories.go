//go:build !docker
// +build !docker

package maddy

var (
	// ConfigDirectory specifies platform-specific value
	// that should be used as a location of default configuration
	//
	// It should not be changed and is defined as a variable
	// only for purposes of modification using -X linker flag.
	ConfigDirectory = "/etc/maddy"

	// DefaultStateDirectory specifies platform-specific
	// default for StateDirectory.
	//
	// Most code should use StateDirectory instead since
	// it will contain the effective location of the state
	// directory.
	//
	// It should not be changed and is defined as a variable
	// only for purposes of modification using -X linker flag.
	DefaultStateDirectory = "/var/lib/maddy"

	// DefaultRuntimeDirectory specifies platform-specific
	// default for RuntimeDirectory.
	//
	// Most code should use RuntimeDirectory instead since
	// it will contain the effective location of the state
	// directory.
	//
	// It should not be changed and is defined as a variable
	// only for purposes of modification using -X linker flag.
	DefaultRuntimeDirectory = "/run/maddy"

	// DefaultLibexecDirectory specifies platform-specific
	// default for LibexecDirectory.
	//
	// Most code should use LibexecDirectory since it will
	// contain the effective location of the libexec
	// directory.
	//
	// It should not be changed and is defined as a variable
	// only for purposes of modification using -X linker flag.
	DefaultLibexecDirectory = "/usr/lib/maddy"
)
