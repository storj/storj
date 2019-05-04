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

type simple struct {
	Str1  string
	Int2  int
	Uint3 uint
}

type nested struct {
	Simple simple
	Int4   int
}

//type nestedPointer struct {
//	Pointer uintptr
//	Simple  simple
//}

var AllTests testing.Tests

func init() {
	AllTests.Register(
		testing.NewTest("TestGoToCStruct_success", TestGoToCStruct_success),
		testing.NewTest("TestGoToCStruct_error", TestGoToCStruct_error),
		testing.NewTest("TestCToGoStruct_success", TestCToGoStruct_success),
		testing.NewTest("TestCToGoStruct_error", TestCToGoStruct_error),
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

	//{
	//	t.Info("go to C struct with pointer")
	//
	//	simpleGo1 := simple{"one", -2, 3,}
	//	simpleGo2 := simple{"two", -10, 5,}
	//	simple1Ptr := reflect.ValueOf(&simpleGo1).Pointer()
	//	nestedPointerGo := nestedPointer{simple1Ptr, simpleGo2}
	//	toCStruct := C.struct_NestedPointer{}
	//
	//	err := GoToCStruct(nestedPointerGo, &toCStruct)
	//	require.NoError(t, err)
	//
	//	assert.Equal(t, nestedPointerGo.Simple.Str1, C.GoString(toCStruct.Simple.Str1))
	//	assert.Equal(t, nestedPointerGo.Simple.Int2, int(toCStruct.Simple.Int2))
	//	assert.Equal(t, nestedPointerGo.Simple.Uint3, uint(toCStruct.Simple.Uint3))
	//
	//	assert.Equal(t, nestedPointerGo.Pointer, uintptr(toCStruct.Pointer))
	//
	//	goValue := reflect.ValueOf(unsafe.Pointer(nestedPointerGo.Pointer))
	//	cValue := reflect.Indirect(reflect.ValueOf(toCStruct.Pointer))
	//	assert.Equal(t, goValue.FieldByName("Str1").String(), simpleGo1.Str1)
	//	assert.Equal(t, cValue.FieldByName("Str1"), simpleGo1.Str1)
	//}
}

func TestGoToCStruct_error(t *testing.T) {
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
	//	t.Info("C to go nested struct")
	//
	//	simpleC := C.struct_Simple{C.CString("two"), -10, 5,}
	//	nestedC := C.struct_Nested{Simple: simpleC, Int4: 4}
	//	toGoStruct := nested{Simple: simple{}}
	//
	//	err := CToGoStruct(nestedC, &toGoStruct)
	//	require.NoError(t, err)
	//
	//	assert.Equal(t, C.GoString(nestedC.Simple.Str1), toGoStruct.Simple.Str1)
	//	assert.Equal(t, int(nestedC.Simple.Int2), toGoStruct.Simple.Int2)
	//	assert.Equal(t, uint(nestedC.Simple.Uint3), toGoStruct.Simple.Uint3)
	//	assert.Equal(t, int(nestedC.Int4), toGoStruct.Int4)
	//}
}

func TestCToGoStruct_error(t *testing.T) {
}
