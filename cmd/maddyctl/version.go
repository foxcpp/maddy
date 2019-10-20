package main

import (
	"fmt"
	"runtime/debug"
)

var Version = "unknown (built from source tree)"

func buildInfo() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version == "(devel)" {
			return Version
		}
		return fmt.Sprintf("%s %s", info.Main.Version, info.Main.Sum)
	}
	return Version
}
