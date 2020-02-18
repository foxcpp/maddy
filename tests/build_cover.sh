#!/bin/sh
exec go test -tags 'cover_main debugflags' -coverpkg 'github.com/foxcpp/maddy,github.com/foxcpp/maddy/pkg/...,github.com/foxcpp/maddy/internal/...' -cover -covermode atomic -c cover_test.go -o maddy.cover
