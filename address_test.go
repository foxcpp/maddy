package maddy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStandardizeAddress(t *testing.T) {
	for _, tc := range []testCase{
		tc("smtp://0.0.0.0:25", "smtp", "0.0.0.0", "25", ""),
		tc("smtp://[::]:25", "smtp", "[::]", "25", ""),
		tc("smtp://0.0.0.0", "smtp", "0.0.0.0", "25", ""),
		tc("smtp://[::]", "smtp", "[::]", "25", ""),
	} {
		out, err := standardizeAddress(tc.in)
		require.NoError(t, err)
		require.Equal(t, tc.expected, out)
	}
}

type testCase struct {
	in string
	expected Address
}

func tc(orig, scheme, host, port, path string) testCase {
	return testCase{
		in: orig,
		expected: Address{
			Original: orig,
			Scheme: scheme,
			Host: host,
			Port: port,
			Path: path,
		},
	}
}
