package main

import (
	"fmt"
	"runtime/debug"
)

const Version = "unknown (built from source tree)"

func printBuildInfo() {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version == "(devel)" {
			fmt.Println("maddy", Version)
		} else {
			fmt.Println("maddy", info.Main.Version, info.Main.Sum)
		}
	} else {
		fmt.Println("maddy", Version, "(GOPATH build)")
		fmt.Println()
		fmt.Println("Building maddy in GOPATH mode can lead to wrong dependency")
		fmt.Println("versions being used. Problems created by this will not be")
		fmt.Println("addressed. Make sure you are building in Module Mode")
		fmt.Println("(see README for details)")
	}
}
