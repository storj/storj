// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMapping_Add(t *testing.T) {
	{
		testMap := newMapping()

		str := "testing 123"
		strToken := testMap.Add(str)

		gotStr, ok := testMap.values[strToken]
		require.True(t, ok)
		assert.Equal(t, str, gotStr)
	}

	{
		testMap := newMapping()

		str := "testing 123"
		strToken := testMap.Add(&str)

		gotStr, ok := testMap.values[strToken]
		require.True(t, ok)
		assert.Equal(t, str, *gotStr.(*string))
	}
}

func TestMapping_Get(t *testing.T) {
	{
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, gotStr)
	}

	{
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = &str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, *gotStr.(*string))
	}
}
