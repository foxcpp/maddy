#!/bin/sh

set -e

./build_cover.sh

clean() {
    rm -f /tmp/maddy-coverage-report*
}
trap clean EXIT

go test -tags integration -integration.executable ./maddy.cover -integration.coverprofile /tmp/maddy-coverage-report "$@"
go run gocovcat.go /tmp/maddy-coverage-report* > coverage.out
