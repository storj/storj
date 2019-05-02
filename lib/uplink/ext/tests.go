// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

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
		//testing.NewTest("TestSanity", TestSanity),
		testing.NewTest("TestConvertStruct_success", TestConvertStruct_success),
		testing.NewTest("TestConvertStruct_error", TestConvertStruct_error),
	)
}

func main() {
	AllTests.Run()
}

//func TestSanity(t *testing.T) {
//	err := errs.New("insane in the membrane")
//	assert.NoError(t, err)
//}

func TestConvertStruct_success(t *testing.T) {
	//stringC := C.CString(stringGo)

	//
	//simpleC := C.struct_SimpleOne{
	//	ballz: true,
	//	//str1: C.CString("one"),
	//	//int2: -2,
	//	//uint: 3,
	//}

	//testCases := []struct {
	//	name     string
	//	from, to interface{}
	//}{
	//	{
	//		name: "strings",
	//		from: stringGo,
	//		to: ,
	//	},
	//	{
	//		name: "strings (reverse)",
	//		from: stringC,
	//		to: stringGo,
	//	},
	//	//{
	//	//	name: "simple struct",
	//	//	from: simpleGo,
	//	//	to:   simpleC,
	//	//},
	//}
	//
	//for _, testCase := range testCases {
	//	t.Info(testCase.name)
	//
	//	err := ConvertStruct()
	//}

	{
		t.Info("go to C string")

		stringGo := "testing 123"
		toCString := C.CString("")

		err := ConvertStruct(stringGo, &toCString)
		require.NoError(t, err)

		assert.Equal(t, stringGo, C.GoString(toCString))
	}

	{
		t.Info("go to C bool")

		boolGo := true
		var toCBool C.bool

		err := ConvertStruct(boolGo, &toCBool)
		require.NoError(t, err)

		assert.Equal(t, boolGo, bool(toCBool))
	}

	{
		t.Info("go to C simple struct")

		simpleGo := struct {
			str1  string
			int2  int
			uint3 uint
		}{"one", -2, 3,}
		toCStruct := C.struct_Simple{}

		err := ConvertStruct(simpleGo, &toCStruct)
		require.NoError(t, err)

		//assert.Equal(t, simpleGo.ballz, cbool)
		//assert.Equal(t, simpleGo.ballz, toCStruct.ballz)
		assert.Equal(t, simpleGo.str1, toCStruct.str1)
		assert.Equal(t, simpleGo.int2, toCStruct.int2)
		assert.Equal(t, simpleGo.uint3, toCStruct.uint3)
	}
}

func TestConvertStruct_error(t *testing.T) {
	//	testCases := struct {
	//		name     string
	//		from, to interface{}
	//	}{
	//
	//	}
}
