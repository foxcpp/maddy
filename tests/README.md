# maddy integration testing

## Tests structure

The test library creates a temporary state and runtime directory, starts the
server with the specified configuration file and lets you interact with it
using a couple of convenient wrappers.

## Running

To run tests, use `go test -tags integration` in this directory. Make sure to
have a maddy executable in the current working directory.
Use `-integration.executable` if the executable is named different or is placed
somewhere else.
Use `-integration.coverprofile` to pass `-test.coverprofile
your_value.RANDOM` to test executable. See `./build_cover.sh` to build a
server executable instrumented with coverage counters.
