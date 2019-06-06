// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapping_Add(t *testing.T) {
	{
		t.Log("string")
		testMap := newMapping()

		str := "testing 123"
		strToken := testMap.Add(str)

		gotStr, ok := testMap.values[strToken]
		require.True(t, ok)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Log("pointer")
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
		t.Log("string")
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Log("pointer")
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = &str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, *gotStr.(*string))
	}
}
