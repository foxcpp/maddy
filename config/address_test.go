package config

import (
	"net"
	"reflect"
	"testing"
)

func TestStandardizeAddress(t *testing.T) {
	for _, expected := range []Address{
		{Original: "smtp://0.0.0.0", Scheme: "smtp", Host: "0.0.0.0", Port: "25"},
		{Original: "smtp://[::]", Scheme: "smtp", Host: "::", Port: "25"},
		{Original: "smtp://0.0.0.0:10025", Scheme: "smtp", Host: "0.0.0.0", Port: "10025"},
		{Original: "smtp://[::]:10025", Scheme: "smtp", Host: "::", Port: "10025"},
	} {
		actual, err := StandardizeAddress(expected.Original)
		if err != nil {
			t.Error("Unexpected failure:", err)
			return
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Didn't parse URL %q correctly", expected.Original)
			return
		}

		addrStr := actual.Address()
		if addrStr != net.JoinHostPort(expected.Host, expected.Port) {
			t.Errorf("Address() returned wrong value: %q for %q", addrStr, expected.Original)
			return
		}
	}
}
