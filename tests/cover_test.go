//+build cover_main

package tests

/*
Go toolchain lacks the ability to instrument arbitrary executables with
coverage counters.

This file wraps the maddy executable into a minimal layer of "test" logic to
make 'go test' work for it and produce the coverage report.

Use ./build_cover.sh to compile it into ./maddy.cover.

References:
https://stackoverflow.com/questions/43381335/how-to-capture-code-coverage-from-a-go-binary
https://blog.cloudflare.com/go-coverage-with-external-tests/
https://github.com/albertito/chasquid/blob/master/coverage_test.go
*/

import (
	"os"
	"testing"

	"github.com/foxcpp/maddy"
)

func TestMain(m *testing.M) {
	// -test.* flags are registered somewhere in init() in "testing" (?)
	// so calling flag.Parse() in maddy.Run() catches them up.

	// maddy.Run changes the working directory, we need to change it back so
	// -test.coverprofile writes out profile in the right location.
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	code := maddy.Run()

	if err := os.Chdir(wd); err != nil {
		panic(err)
	}

	// Silence output produced by "testing" runtime.
	_, w, err := os.Pipe()
	if err == nil {
		os.Stderr = w
		os.Stdout = w
	}

	// Even though we do not have any tests to run, we need to call out into
	// "testing" to make it process flags and produce the coverage report.
	m.Run()

	// TestMain doc says we have to exit with a sensible status code on our
	// own.
	os.Exit(code)
}
