// +build ignore

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
		t.Log("string")
		testMap := NewMapping()

		str := "testing 123"
		strToken := testMap.Add(str)

		gotStr, ok := testMap.values[strToken]
		require.True(t, ok)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Log("pointer")
		testMap := NewMapping()

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
		testMap := NewMapping()

		str := "testing 123"
		strToken := Token(1)
		testMap.values[strToken] = str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Log("pointer")
		testMap := NewMapping()

		str := "testing 123"
		strToken := Token(1)
		testMap.values[strToken] = &str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, *gotStr.(*string))
	}
}
