package main

import (
	"fmt"
	"runtime/debug"
)

const Version = "unknown (built from source tree)"

func buildInfo() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		imapsqlVer := "unknown"
		for _, dep := range info.Deps {
			if dep.Path == "github.com/foxcpp/go-imap-sql" {
				imapsqlVer = dep.Version
			}
		}

		if info.Main.Version == "(devel)" {
			return fmt.Sprintf("%s for maddy %s", imapsqlVer, Version)
		}
		return fmt.Sprintf("%s for maddy %s %s", imapsqlVer, info.Main.Version, info.Main.Sum)
	}
	return fmt.Sprintf("unknown for maddy %s (GOPATH build)", Version)
}
