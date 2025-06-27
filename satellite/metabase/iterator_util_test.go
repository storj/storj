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

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a"},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "a"}, ""))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "a/"}, ""))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "a/x/y"}, ""))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "a/x/y"},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "a/x/y"}, "a/x/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "2017/05/08" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "2017/05/08/"}, "2017/05/"))

	assert.Equal(t,
		metabase.ObjectsIteratorCursor{Key: "2017/05/08" + afterDelimiter, Version: metabase.MaxVersion, Inclusive: true},
		metabase.FirstIterateCursor(false, metabase.IterateCursor{Key: "2017/05/08/x/y"}, "2017/05/"))
}
