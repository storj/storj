// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

//go:generate go run .

import "C"

// NB: standard go tests cannot import "C"

// #cgo CFLAGS: -g -Wall
// #include "example/test.h"
import "C"
import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink/ext/testing"
)

var AllTests testing.Tests

func init() {
	AllTests.Register(
		testing.NewTest("TestGoToCStruct_success", TestGoToCStruct_success),
		testing.NewTest("TestGoToCStruct_error", TestGoToCStruct_error),
	)
}

func main() {
	AllTests.Run()
}

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

		simpleGo := struct {
			Str1  string
			Int2  int
			Uint3 uint
		}{"one", -2, 3,}
		toCStruct := C.struct_Simple{}

		err := GoToCStruct(simpleGo, &toCStruct)
		require.NoError(t, err)

		assert.Equal(t, simpleGo.Str1, C.GoString(toCStruct.Str1))
		assert.Equal(t, simpleGo.Int2, int(toCStruct.Int2))
		assert.Equal(t, simpleGo.Uint3, uint(toCStruct.Uint3))
	}
}

func TestGoToCStruct_error(t *testing.T) {
}
