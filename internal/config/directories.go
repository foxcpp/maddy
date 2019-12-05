package config

var (
	// StateDirectory contains the path to the directory that
	// should be used to store any data that should be
	// preserved between sessions.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	StateDirectory string

	// RuntimeDirectory contains the path to the directory that
	// should be used to store any temporary data.
	//
	// It should be preferred over os.TempDir, which is
	// global and world-readable on most systems, while
	// RuntimeDirectory can be dedicated for maddy.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	RuntimeDirectory string

	// LibexecDirectory contains the path to the directory
	// where helper binaries should be searched.
	//
	// Value of this variable must not change after initialization
	// in cmd/maddy/main.go.
	LibexecDirectory string
)
