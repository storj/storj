// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefixLimit(t *testing.T) {
	unchanged := ObjectKey("unchanged")
	_ = prefixLimit(unchanged)
	require.Equal(t, ObjectKey("unchanged"), unchanged)

	tests := []struct{ in, exp ObjectKey }{
		{"", ""},
		{"a", "b"},
		{"\xF1", "\xF2"},
		{"\xFF", "\xFF\x00"},
	}
	for _, test := range tests {
		require.Equal(t, test.exp, prefixLimit(test.in))
		if test.in != "" {
			require.True(t, lessKey(test.in, test.exp))
		}
	}
}
