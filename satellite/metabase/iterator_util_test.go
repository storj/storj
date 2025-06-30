// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/metabase"
)

func TestFirstIterateCursor(t *testing.T) {
	afterDelimiter := metabase.ObjectKey('/' + 1)

	firstIterateCursor := func(key, prefix, delimiter metabase.ObjectKey) metabase.ObjectsIteratorCursor {
		t.Helper()
		c, ok := metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: key}, prefix, delimiter)
		assert.True(t, ok)
		return c
	}

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a"},
		firstIterateCursor("a", "", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("a/", "", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("a/x/y", "", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a/x/y"},
		firstIterateCursor("a/x/y", "a/x/", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "2017/05/08" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("2017/05/08/", "2017/05/", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "2017/05/08" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("2017/05/08/x/y", "2017/05/", "/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "\xFF\xF1", Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("\xFF\xF0ABC", "", "\xFF\xF0"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "2017\xFF\xFF\xF1", Version: metabase.MaxVersion, Inclusive: true},
		firstIterateCursor("2017\xFF\xFF\xF0ABC", "2017\xFF", "\xFF\xF0"))

	noFirstIterateCursor := func(key, prefix, delimiter metabase.ObjectKey) {
		t.Helper()
		_, ok := metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: key}, prefix, delimiter)
		assert.False(t, ok)
	}

	noFirstIterateCursor("2017\xFF\xFF", "2017", "\xFF")
	noFirstIterateCursor("2017\xFF\xFFA", "2017\xFF", "\xFF")
}
