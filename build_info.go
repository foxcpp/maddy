package maddy

import (
	"runtime/debug"
)

var Version = "unknown (built from source tree)"

func BuildInfo() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version == "(devel)" {
			return Version
		}
		return info.Main.Version + " " + info.Main.Sum
	}
	return Version + " (GOPATH build)"
}
