// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniverse(t *testing.T) {
	{
		handles := newHandles()

		str := "testing 123"
		handle := handles.Add(str)

		got := handles.Get(handle)
		assert.Equal(t, str, got)

		handles.Del(handle)
		assert.True(t, handles.Empty())
	}

	{
		handles := newHandles()

		str := "testing 123"
		handle := handles.Add(&str)

		got := handles.Get(handle)
		assert.Equal(t, str, *got.(*string))

		handles.Del(handle)
		assert.True(t, handles.Empty())
		assert.Nil(t, handles.Get(handle))
	}
}
