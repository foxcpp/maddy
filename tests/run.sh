#!/bin/sh

set -e

if [ -z "$GO" ]; then
	export GO=go
fi

./build_cover.sh

clean() {
    rm -f /tmp/maddy-coverage-report*
}
trap clean EXIT

$GO test -tags integration -integration.executable ./maddy.cover -integration.coverprofile /tmp/maddy-coverage-report "$@"
$GO run gocovcat.go /tmp/maddy-coverage-report* > coverage.out
