// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main
// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include "example/test.h"
import "C"
import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink/ext/testing"
)

func TestGoToCStruct_success(t *testing.T) {
	{
		t.Info("go to C string")

		stringGo := "testing 123"
		toCString := C.CString("")

		err := GoToCStruct(stringGo, &toCString)
		require.NoError(t, err)

		assert.Equal(t, stringGo, C.GoString(toCString))
	}

	{
		t.Info("go to C bool")

		boolGo := true
		var toCBool C.bool

		err := GoToCStruct(boolGo, &toCBool)
		require.NoError(t, err)

		assert.Equal(t, boolGo, bool(toCBool))
	}

	{
		t.Info("go to C simple struct")

		simpleGo := simple{"one", -2, 3,}
		toCStruct := C.struct_Simple{}

		err := GoToCStruct(simpleGo, &toCStruct)
		require.NoError(t, err)

		assert.Equal(t, simpleGo.Str1, C.GoString(toCStruct.Str1))
		assert.Equal(t, simpleGo.Int2, int(toCStruct.Int2))
		assert.Equal(t, simpleGo.Uint3, uint(toCStruct.Uint3))
	}

	{
		t.Info("go to C nested struct")

		simpleGo := simple{"two", -10, 5,}
		nestedGo := nested{simpleGo, 4}
		toCStruct := C.struct_Nested{}

		err := GoToCStruct(nestedGo, &toCStruct)
		require.NoError(t, err)

		assert.Equal(t, nestedGo.Simple.Str1, C.GoString(toCStruct.Simple.Str1))
		assert.Equal(t, nestedGo.Simple.Int2, int(toCStruct.Simple.Int2))
		assert.Equal(t, nestedGo.Simple.Uint3, uint(toCStruct.Simple.Uint3))
		assert.Equal(t, nestedGo.Int4, int(toCStruct.Int4))
	}
}

func TestGoToCStruct_error(t *testing.T) {
	// TODO
}

func TestCToGoStruct_success(t *testing.T) {
	{
		t.Info("C to go string")

		stringC := C.CString("testing 123")
		toGoString := ""

		err := CToGoStruct(stringC, &toGoString)
		require.NoError(t, err)

		assert.Equal(t, C.GoString(stringC), toGoString)
	}

	{
		t.Info("C to go bool")

		boolC := C.bool(true)
		toGoBool := false

		err := CToGoStruct(boolC, &toGoBool)
		require.NoError(t, err)

		assert.Equal(t, bool(boolC), toGoBool)
	}

	{
		t.Info("C to go simple struct")

		simpleC := C.struct_Simple{C.CString("one"), -2, 3,}
		toGoStruct := simple{}

		err := CToGoStruct(simpleC, &toGoStruct)
		require.NoError(t, err)

		assert.Equal(t, C.GoString(simpleC.Str1), toGoStruct.Str1)
		assert.Equal(t, int(simpleC.Int2), toGoStruct.Int2)
		assert.Equal(t, uint(simpleC.Uint3), toGoStruct.Uint3)
	}

	//{
	//	t.Info("C to go uchar")
	//
	//	ucharC := C.uchar('f')
	//	var toGoChar rune
	//
	//	err := CToGoStruct(ucharC, &toGoChar)
	//	require.NoError(t, err)
	//
	//	assert.Equal(t, rune(ucharC), toGoChar)
	//}
}

func TestCToGoStruct_error(t *testing.T) {
	// TODO
}
