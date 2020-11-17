// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextPrefix(t *testing.T) {
	unchanged := ObjectKey("unchanged")
	_ = nextPrefix(unchanged)
	require.Equal(t, ObjectKey("unchanged"), unchanged)

	tests := []struct{ in, exp ObjectKey }{
		{"", ""},
		{"a", "b"},
		{"\xF1", "\xF2"},
	}
	for _, test := range tests {
		require.Equal(t, test.exp, nextPrefix(test.in))
		if test.in != "" {
			require.True(t, lessKey(test.in, test.exp))
		}
	}
}

func TestBeforeKey(t *testing.T) {
	unchanged := ObjectKey("unchanged")
	_ = beforeKey(unchanged)
	require.Equal(t, ObjectKey("unchanged"), unchanged)

	tests := []struct{ in, exp ObjectKey }{
		{"", ""},
		{"b", "a\xFF"},
		{"\xF1", "\xF0\xFF"},
	}
	for _, test := range tests {
		require.Equal(t, test.exp, beforeKey(test.in))
		if test.in != "" {
			require.True(t, lessKey(test.exp, test.in))
		}
	}
}
