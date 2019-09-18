//+build !go1.12

package main

import (
	"fmt"
)

func printBuildInfo() {
	fmt.Println("maddy", Version)
}
