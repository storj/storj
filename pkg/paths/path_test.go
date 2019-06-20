// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertConsumed(t *testing.T, it Iterator, consumed string, ok bool) {
	t.Helper()
	gotConsumed, gotOk := it.Consumed()
	assert.Equal(t, gotConsumed, consumed)
	assert.Equal(t, gotOk, ok)
}

func TestUnencrypted(t *testing.T) {
	it := NewUnencrypted("foo").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assertConsumed(t, it, "foo", true)
	assert.True(t, it.Done())

	it = NewUnencrypted("").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "", true)
	assert.True(t, it.Done())

	it = NewUnencrypted("foo/").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assertConsumed(t, it, "foo/", true)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "foo/", true)
	assert.True(t, it.Done())

	it = Unencrypted{}.Iterator()
	assertConsumed(t, it, "", false)
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "", false)
}

func TestEncrypted(t *testing.T) {
	it := NewEncrypted("foo").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assertConsumed(t, it, "foo", true)
	assert.True(t, it.Done())

	it = NewEncrypted("").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "", true)
	assert.True(t, it.Done())

	it = NewEncrypted("foo/").Iterator()
	assertConsumed(t, it, "", false)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assertConsumed(t, it, "foo/", true)
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "foo/", true)
	assert.True(t, it.Done())

	it = Encrypted{}.Iterator()
	assertConsumed(t, it, "", false)
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assertConsumed(t, it, "", false)
}

func TestIterator(t *testing.T) {
	for i, tt := range []struct {
		path  string
		comps []string
	}{
		{"", []string{""}},
		{"/", []string{"", ""}},
		{"//", []string{"", "", ""}},
		{" ", []string{" "}},
		{"a", []string{"a"}},
		{"/a/", []string{"", "a", ""}},
		{"a/b/c/d", []string{"a", "b", "c", "d"}},
		{"///a//b////c/d///", []string{"", "", "", "a", "", "b", "", "", "", "c", "d", "", "", ""}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		iter, got := NewIterator(tt.path), make([]string, 0, len(tt.comps))
		for !iter.Done() {
			got = append(got, iter.Next())
		}
		assert.Equal(t, tt.comps, got, errTag)
	}
}

func TestIteratorConsumed(t *testing.T) {
	it := NewIterator("foo")
	consumed, ok := it.Consumed()
	assert.Equal(t, consumed, "")
	assert.False(t, ok)
	assert.Equal(t, it.Next(), "foo")
	consumed, ok = it.Consumed()
	assert.Equal(t, consumed, "foo")
	assert.True(t, ok)
}
