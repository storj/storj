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
		ref := universe.Add(str)

		got := universe.Get(ref)
		assert.Equal(t, str, got)
	}

	{
		universe := NewUniverse()

		str := "testing 123"
		ref := universe.Add(&str)

		got := universe.Get(ref)
		assert.Equal(t, str, *got.(*string))

		universe.Del(ref)

		got = universe.Get(ref)
		assert.Nil(t, got)
	}
}
