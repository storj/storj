// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniverse(t *testing.T) {
	{
		universe := NewUniverse()

		str := "testing 123"
		handle := universe.Add(str)

		got := universe.Get(handle)
		assert.Equal(t, str, got)
	}

	{
		universe := NewUniverse()

		str := "testing 123"
		handle := universe.Add(&str)

		got := universe.Get(handle)
		assert.Equal(t, str, *got.(*string))

		universe.Del(&handle)
		assert.Zero(t, handle)

		got = universe.Get(handle)
		assert.Nil(t, got)
	}
}
