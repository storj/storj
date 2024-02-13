// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestFirstIterateCursor(t *testing.T) {
	afterDelimiter := ObjectKey('/' + 1)

	assert.Equal(t,
		iterateCursor{Key: "a"},
		firstIterateCursor(false, IterateCursor{Key: "a"}, ""))

	assert.Equal(t,
		iterateCursor{Key: "a" + afterDelimiter, Version: MaxVersion, Inclusive: true},
		firstIterateCursor(false, IterateCursor{Key: "a/"}, ""))

	assert.Equal(t,
		iterateCursor{Key: "a" + afterDelimiter, Version: MaxVersion, Inclusive: true},
		firstIterateCursor(false, IterateCursor{Key: "a/x/y"}, ""))

	assert.Equal(t,
		iterateCursor{Key: "a/x/y"},
		firstIterateCursor(false, IterateCursor{Key: "a/x/y"}, "a/x/"))

	assert.Equal(t,
		iterateCursor{Key: "2017/05/08" + afterDelimiter, Version: MaxVersion, Inclusive: true},
		firstIterateCursor(false, IterateCursor{Key: "2017/05/08/"}, "2017/05/"))

	assert.Equal(t,
		iterateCursor{Key: "2017/05/08" + afterDelimiter, Version: MaxVersion, Inclusive: true},
		firstIterateCursor(false, IterateCursor{Key: "2017/05/08/x/y"}, "2017/05/"))
}
