package config

import (
	"reflect"
	"testing"
)

func TestStandardizeAddress(t *testing.T) {
	for _, expected := range []Endpoint{
		{Original: "tcp://0.0.0.0:10025", Scheme: "tcp", Host: "0.0.0.0", Port: "10025"},
		{Original: "tcp://[::]:10025", Scheme: "tcp", Host: "::", Port: "10025"},
		{Original: "unix://path", Scheme: "unix", Host: "", Path: "path", Port: ""},
		{Original: "unix:///path", Scheme: "unix", Host: "", Path: "/path", Port: ""},
		{Original: "unix://also/path", Scheme: "unix", Host: "", Path: "also/path", Port: ""},
		{Original: "unix:///also/path", Scheme: "unix", Host: "", Path: "/also/path", Port: ""},
		{Original: "tls://0.0.0.0:10025", Scheme: "tls", Host: "0.0.0.0", Port: "10025"},
	} {
		actual, err := ParseEndpoint(expected.Original)
		if err != nil {
			t.Error("Unexpected failure:", err)
			return
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Didn't parse URL %q correctly\ngot %#v\nwant %#v", expected.Original, actual, expected)
			continue
		}

		if actual.String() != expected.Original {
			t.Errorf("actual.String() = %s, want %s", actual.String(), expected.Original)
		}
	}
}
