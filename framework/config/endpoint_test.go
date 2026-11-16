/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2020 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpoint_NetworkAddressTLS(t *testing.T) {
	for _, case_ := range []struct {
		endpoint Endpoint
		network  string
		address  string
		isTLS    bool
		isFD     bool
	}{
		{
			endpoint: Endpoint{
				Original: "tcp://0.0.0.0:10025",
				Scheme:   "tcp",
				Host:     "0.0.0.0",
				Port:     "10025",
			},
			network: "tcp",
			address: "0.0.0.0:10025",
			isTLS:   false,
			isFD:    false,
		},
		{
			endpoint: Endpoint{
				Original: "tcp://0.0.0.0:10025",
				Scheme:   "tcp4",
				Host:     "0.0.0.0",
				Port:     "10025",
			},
			network: "tcp4",
			address: "0.0.0.0:10025",
			isTLS:   false,
			isFD:    false,
		},
		{
			endpoint: Endpoint{
				Original: "tcp://0.0.0.0:10025",
				Scheme:   "tcp6",
				Host:     "0.0.0.0",
				Port:     "10025",
			},
			network: "tcp6",
			address: "0.0.0.0:10025",
			isTLS:   false,
			isFD:    false,
		},
		{
			endpoint: Endpoint{
				Original:       "tls://0.0.0.0:10025",
				Scheme:         "tcp",
				SchemeModifier: "tls",
				Host:           "0.0.0.0",
				Port:           "10025",
			},
			network: "tcp",
			address: "0.0.0.0:10025",
			isTLS:   true,
			isFD:    false,
		},
		{
			endpoint: Endpoint{
				Original:       "tls://0.0.0.0:10025",
				Scheme:         "tcp4",
				SchemeModifier: "tls",
				Host:           "0.0.0.0",
				Port:           "10025",
			},
			network: "tcp4",
			address: "0.0.0.0:10025",
			isTLS:   true,
			isFD:    false,
		},
		{
			endpoint: Endpoint{
				Original: "fd:3",
				Scheme:   "fd",
				Path:     "3",
			},
			network: "fd",
			address: "3",
			isTLS:   false,
			isFD:    true,
		},
	} {
		require.Equal(t, case_.network, case_.endpoint.Network())
		require.Equal(t, case_.address, case_.endpoint.Address())
		require.Equal(t, case_.isTLS, case_.endpoint.IsTLS())
		require.Equal(t, case_.isFD, case_.endpoint.IsFD())
	}
}

func TestStandardizeAddress(t *testing.T) {
	for _, expected := range []Endpoint{
		{
			Original: "tcp://0.0.0.0:10025",
			Scheme:   "tcp",
			Host:     "0.0.0.0",
			Port:     "10025",
		},
		{
			Original: "tcp4://0.0.0.0:10025",
			Scheme:   "tcp4",
			Host:     "0.0.0.0",
			Port:     "10025",
		},
		{
			Original: "tcp6://0.0.0.0:10025",
			Scheme:   "tcp6",
			Host:     "0.0.0.0",
			Port:     "10025",
		},
		{
			Original: "tcp://[::]:10025",
			Scheme:   "tcp",
			Host:     "::",
			Port:     "10025",
		},
		{
			Original: "tcp:127.0.0.1:10025",
			Scheme:   "tcp",
			Host:     "127.0.0.1",
			Port:     "10025",
		},
		{
			Original: "unix://path",
			Scheme:   "unix",
			Host:     "",
			Path:     "path",
			Port:     "",
		},
		{
			Original: "unix:path",
			Scheme:   "unix",
			Host:     "",
			Path:     "path",
			Port:     "",
		},
		{
			Original: "unix:/path",
			Scheme:   "unix",
			Host:     "",
			Path:     "/path",
			Port:     "",
		},
		{
			Original: "unix:///path",
			Scheme:   "unix",
			Host:     "",
			Path:     "/path",
			Port:     "",
		},
		{
			Original: "unix://also/path",
			Scheme:   "unix",
			Host:     "",
			Path:     "also/path",
			Port:     "",
		},
		{
			Original:       "unix+tls:///path-with-tls",
			Scheme:         "unix",
			SchemeModifier: "tls",
			Host:           "",
			Path:           "/path-with-tls",
			Port:           "",
		},
		{
			Original: "unix:///also/path",
			Scheme:   "unix",
			Host:     "",
			Path:     "/also/path",
			Port:     "",
		},
		{
			Original:       "tls://0.0.0.0:10025",
			Scheme:         "tcp",
			SchemeModifier: "tls",
			Host:           "0.0.0.0",
			Port:           "10025",
		},
		{
			Original:       "tcp+tls://0.0.0.0:10025",
			Scheme:         "tcp",
			SchemeModifier: "tls",
			Host:           "0.0.0.0",
			Port:           "10025",
		},
		{
			Original:       "tls:0.0.0.0:10025",
			Scheme:         "tcp",
			SchemeModifier: "tls",
			Host:           "0.0.0.0",
			Port:           "10025",
		},
		{
			Original: "fd:3",
			Scheme:   "fd",
			Path:     "3",
		},
		{
			Original: "fd://3",
			Scheme:   "fd",
			Path:     "3",
		},
		{
			Original: "fdname:named_fd",
			Scheme:   "fdname",
			Path:     "named_fd",
		},
		{
			Original: "fdname://named_fd",
			Scheme:   "fdname",
			Path:     "named_fd",
		},
		{
			Original:       "fd+tls:3",
			Scheme:         "fd",
			SchemeModifier: "tls",
			Path:           "3",
		},
		{
			Original:       "fdname+tls:named_tls_fd",
			Scheme:         "fdname",
			SchemeModifier: "tls",
			Path:           "named_tls_fd",
		},
	} {
		actual, err := ParseEndpoint(expected.Original)
		if err != nil {
			t.Errorf("Unexpected failure for %s: %v", expected.Original, err)
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
