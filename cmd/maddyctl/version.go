package main

import (
	"fmt"
	"runtime/debug"
)

func buildInfo() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version == "(devel)" {
			return "unknown (built from source tree)"
		}
		return fmt.Sprintf("%s %s", info.Main.Version, info.Main.Sum)
	}
	return "unknown (GOPATH build)"
}
