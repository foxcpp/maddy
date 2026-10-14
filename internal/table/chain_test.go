/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2026 Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package table

import (
	"context"
	"errors"
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainLookupMulti(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("lookup failed")
	tests := []struct {
		name     string
		chain    []module.Table
		optional []bool
		key      string
		want     []string
		wantErr  error
	}{
		{
			name: "passes value through each step",
			chain: []module.Table{
				testutils.Table{M: map[string]string{"user+tag@example.org": "user@example.org"}},
				testutils.Table{M: map[string]string{"user@example.org": "account"}},
			},
			optional: []bool{false, false},
			key:      "user+tag@example.org",
			want:     []string{"account"},
		},
		{
			name: "looks up every value returned by multi table",
			chain: []module.Table{
				testutils.MultiTable{M: map[string][]string{
					"team@example.org": {"alice@example.org", "bob@example.org"},
				}},
				testutils.Table{M: map[string]string{
					"alice@example.org": "alice",
					"bob@example.org":   "bob",
				}},
			},
			optional: []bool{false, false},
			key:      "team@example.org",
			want:     []string{"alice", "bob"},
		},
		{
			name: "looks up every value across consecutive multi tables",
			chain: []module.Table{
				testutils.MultiTable{M: map[string][]string{
					"team@example.org": {"engineering", "support"},
				}},
				testutils.MultiTable{M: map[string][]string{
					"engineering": {"alice", "bob"},
					"support":     {"carol", "dave"},
				}},
			},
			optional: []bool{false, false},
			key:      "team@example.org",
			want:     []string{"alice", "bob", "carol", "dave"},
		},
		{
			name: "required step does not match",
			chain: []module.Table{
				testutils.Table{M: map[string]string{"known": "value"}},
			},
			optional: []bool{false},
			key:      "unknown",
			want:     []string{},
		},
		{
			name: "optional step passes original value to next step",
			chain: []module.Table{
				testutils.Table{M: map[string]string{"alias@example.org": "account"}},
				testutils.Table{M: map[string]string{"user@example.org": "user"}},
			},
			optional: []bool{true, false},
			key:      "user@example.org",
			want:     []string{"user"},
		},
		{
			name: "optional step uses mapped value when it matches",
			chain: []module.Table{
				testutils.Table{M: map[string]string{"alias@example.org": "user@example.org"}},
				testutils.Table{M: map[string]string{"user@example.org": "user"}},
			},
			optional: []bool{true, false},
			key:      "alias@example.org",
			want:     []string{"user"},
		},
		{
			name: "optional multi table passes value through on empty result",
			chain: []module.Table{
				testutils.MultiTable{M: map[string][]string{
					"user@example.org": {},
				}},
				testutils.Table{M: map[string]string{
					"user@example.org": "user",
				}},
			},
			optional: []bool{true, false},
			key:      "user@example.org",
			want:     []string{"user"},
		},
		{
			name: "returns lookup error",
			chain: []module.Table{
				testutils.Table{Err: lookupErr},
			},
			optional: []bool{false},
			key:      "user@example.org",
			want:     []string{},
			wantErr:  lookupErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chain := Chain{chain: tc.chain, optional: tc.optional}
			got, err := chain.LookupMulti(context.Background(), tc.key)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestChainLookup(t *testing.T) {
	t.Parallel()

	chain := Chain{
		chain: []module.Table{
			testutils.Table{M: map[string]string{"alias@example.org": "user@example.org"}},
		},
		optional: []bool{false},
	}

	value, ok, err := chain.Lookup(context.Background(), "alias@example.org")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "user@example.org", value)

	value, ok, err = chain.Lookup(context.Background(), "unknown@example.org")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, value)
}
