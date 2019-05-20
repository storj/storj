// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include "c/tests/test.h"
// #include "c/headers/main.h"
import "C"
import (
	"unsafe"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/lib/uplink/ext/pb"
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
}

func TestCToGoStruct_error(t *testing.T) {
	// TODO
}

func TestSendToGo_success(t *testing.T) {
	{
		t.Info("uplink config")

		startConfig := &pb.UplinkConfig{
			// -- WIP | TODO
			Tls: &pb.TLSConfig{
				SkipPeerCaWhitelist: true,
				PeerCaWhitelistPath: "/whitelist.pem",
			},
			IdentityVersion: &pb.IDVersion{
				Number: 0,
			},
			MaxInlineSize: 1,
			MaxMemory: 2,
		}
		snapshot, err := proto.Marshal(startConfig)
		require.NoError(t, err)
		require.NotEmpty(t, snapshot)

		// NB/TODO: I don't think this is exactly right but might work
		size := uintptr(len(snapshot))

		cVal := &C.struct_GoValue{
			//Ptr: (0 by default),
			Type: C.UplinkConfigType,
			Snapshot:  (*C.uchar)(unsafe.Pointer(&snapshot)),
			Size:      C.ulong(size),
		}
		assert.Zero(t, cVal.Ptr)

		cErr := C.CString("")
		SendToGo(cVal, &cErr)
		require.Empty(t, C.GoString(cErr))

		assert.NotZero(t, uintptr(cVal.Ptr))
		assert.NotZero(t, cVal.Type)


		//value := CToGoGoValue(*cVal)
		//endConfig := structRefMap.Get(token(value.ptr))
		////endConfig := (*pb.UplinkConfig)(unsafe.Pointer(value.ptr))
		//
		//startJSON, err := json.MarshalIndent(startConfig, "", "  ")
		//require.NoError(t, err)
		//
		//endJSON, err := json.MarshalIndent(endConfig, "", "  ")
		//require.NoError(t, err)

		//_, diff := jsondiff.Compare(startJSON, endJSON, nil)
		//t.Debug("", zap.String("diff", diff))
		//t.Info("", zap.String("startConfig", string(startJSON)))
		//t.Info("",
		//	zap.Any("value.Ptr", value.ptr),
		//	//zap.Uintptr("unsafe", unsafe.Pointer(value.ptr)),
		//	zap.Any("get", structRefMap.Get(token(value.ptr))),
		//	zap.Any("cast", (structRefMap.Get(token(value.ptr))).(*pb.UplinkConfig)),
		//)
		//t.Info("", zap.String("endConfig", string(endJSON)))
		//assert.True(t, reflect.DeepEqual(startConfig, endConfig))
		//assert.Equal(t, *(startConfig.Tls), *(endConfig.Tls))
		//assert.Equal(t, *(startConfig.IdentityVersion), *(endConfig.IdentityVersion))
	}

	// TODO: other types
}

func TestSendToGo_error(t *testing.T) {
	// TODO
}

func TestCToGoGoValue(t *testing.T) {
	//str := "test string 123"
	//cVal := C.struct_GoValue{
	//	Ptr: C.GoUintptr(uintptr(unsafe.Pointer(&str))),
	//	// NB: arbitrary type
	//	Type: C.APIKeyType,
	//}
	//
	//value := CToGoGoValue(cVal)
	//assert.Equal(t, uint(cVal.Type), value._type)
	//assert.NotZero(t, value.ptr)
	//
	//strPtr, ok := structRefMap.Get(token(value.ptr)).(*string)
	//require.True(t, ok)
	//assert.Equal(t, str, *strPtr)
}

func TestMapping_Add(t *testing.T) {
	testMap := newMapping()

	str := "testing 123"
	strToken := testMap.Add(str)

	gotStr, ok := testMap.values[strToken]
	require.True(t, ok)
	assert.Equal(t, str, gotStr)
}

func TestMapping_Get(t *testing.T) {
	testMap := newMapping()

	str := "testing 123"
	strToken := token(1)
	testMap.values[strToken] = str

	gotStr := testMap.Get(strToken)
	assert.Equal(t, str, gotStr)
}
