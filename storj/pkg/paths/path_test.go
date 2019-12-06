// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnencrypted(t *testing.T) {
	it := NewUnencrypted("foo").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assert.Equal(t, it.Consumed(), "foo")
	assert.True(t, it.Done())

	it = NewUnencrypted("").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "")

	it = NewUnencrypted("foo/").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assert.Equal(t, it.Consumed(), "foo/")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "foo/")
	assert.True(t, it.Done())

	it = Unencrypted{}.Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "")
}

func TestEncrypted(t *testing.T) {
	it := NewEncrypted("foo").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assert.Equal(t, it.Consumed(), "foo")
	assert.True(t, it.Done())

	it = NewEncrypted("").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "")

	it = NewEncrypted("foo/").Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "foo")
	assert.Equal(t, it.Consumed(), "foo/")
	assert.False(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "foo/")
	assert.True(t, it.Done())

	it = Encrypted{}.Iterator()
	assert.Equal(t, it.Consumed(), "")
	assert.True(t, it.Done())
	assert.Equal(t, it.Next(), "")
	assert.Equal(t, it.Consumed(), "")
}

func TestIterator(t *testing.T) {
	for i, tt := range []struct {
		path  string
		comps []string
	}{
		{"", []string{}},
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
