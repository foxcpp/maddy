//go:build docker
// +build docker

package maddy

var (
	ConfigDirectory         = "/data"
	DefaultStateDirectory   = "/data"
	DefaultRuntimeDirectory = "/tmp"
	DefaultLibexecDirectory = "/usr/lib/maddy"
)
