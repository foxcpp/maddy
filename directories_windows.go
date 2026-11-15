//go:build windows && !docker
// +build windows,!docker

package maddy

import (
	"os"
	"path/filepath"
)

const (
	defaultProgramData  = `C:\ProgramData`
	defaultProgramFiles = `C:\Program Files`
)

func programData() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return dir
	}
	return defaultProgramData
}

func programFiles() string {
	if dir := os.Getenv("ProgramFiles"); dir != "" {
		return dir
	}
	return defaultProgramFiles
}

var (
	// ConfigDirectory specifies platform-specific value
	// that should be used as a location of default configuration.
	ConfigDirectory = filepath.Join(programData(), `maddy`)

	// DefaultStateDirectory specifies platform-specific
	// default for StateDirectory.
	//
	// Most code should use StateDirectory instead since
	// it will contain the effective location of the state
	// directory.
	DefaultStateDirectory = filepath.Join(programData(), `maddy`, `data`)

	// DefaultRuntimeDirectory specifies platform-specific
	// default for RuntimeDirectory.
	//
	// Most code should use RuntimeDirectory instead since
	// it will contain the effective location of the runtime
	// directory.
	DefaultRuntimeDirectory = filepath.Join(programData(), `maddy`, `run`)

	// DefaultLibexecDirectory specifies platform-specific
	// default for LibexecDirectory.
	//
	// Most code should use LibexecDirectory since it will
	// contain the effective location of the libexec
	// directory.
	DefaultLibexecDirectory = filepath.Join(programFiles(), `maddy`)
)
