//+build go1.12

package main

import (
	"fmt"
	"runtime/debug"
)

func printBuildInfo() {
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Println("maddy", Version)
		fmt.Println()
		fmt.Println("Module version:", info.Main.Version)
		if info.Main.Sum != "" {
			fmt.Println("Module checksum:", info.Main.Sum)
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
