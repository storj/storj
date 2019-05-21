// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include "c/tests/test.h"
// #include "c/headers/main.h"
import "C"
import (
	"encoding/json"
	"github.com/nsf/jsondiff"
	"unsafe"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/lib/uplink/ext/pb"
	"storj.io/storj/lib/uplink/ext/testing"
)

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
			MaxMemory:     2,
		}
		snapshot, err := proto.Marshal(startConfig)
		require.NoError(t, err)
		require.NotEmpty(t, snapshot)

		// NB/TODO: I don't think this is exactly right but might work
		size := uintptr(len(snapshot))

		t.Info("", zap.ByteString("snapshot 1", snapshot))
		cVal := &C.struct_GoValue{
			//Ptr: (0 by default),
			Type:     C.UplinkConfigType,
			Snapshot: (*C.uchar)(unsafe.Pointer(&snapshot)),
			Size:     C.ulong(size),
		}
		t.Info("", zap.ByteString("snapshot 2", *(*[]byte)(unsafe.Pointer(cVal.Snapshot))))
		assert.Zero(t, cVal.Ptr)

		cErr := C.CString("")
		SendToGo(cVal, &cErr)
		//t.Info("", zap.ByteString("snapshot 3", *(*[]byte)(unsafe.Pointer(cVal.Snapshot))))
		require.Empty(t, C.GoString(cErr))

		assert.NotZero(t, uintptr(cVal.Ptr))
		assert.NotZero(t, cVal.Type)

		//value := CToGoGoValue(*cVal)
		//endConfig := structRefMap.Get(token(value.ptr))
		//require.Equal(t, uintptr(cVal.Ptr), value.ptr)
		endConfig := structRefMap.Get(token(cVal.Ptr))

		startJSON, err := json.Marshal(startConfig)
		require.NoError(t, err)

		endJSON, err := json.Marshal(endConfig)
		require.NoError(t, err)

		match, diffStr := jsondiff.Compare(startJSON, endJSON, &jsondiff.Options{})
		if !assert.Equal(t, jsondiff.FullMatch, match) {
			t.Error("config JSON diff:", zap.String("", diffStr))
		}
	}

	// TODO: other types
}

func TestSendToGo_error(t *testing.T) {
	// TODO
}

func TestMapping_Add(t *testing.T) {
	{
		t.Info("string")
		testMap := newMapping()

		str := "testing 123"
		strToken := testMap.Add(str)

		gotStr, ok := testMap.values[strToken]
		require.True(t, ok)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Info("pointer")
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
		t.Info("string")
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, gotStr)
	}

	{
		t.Info("pointer")
		testMap := newMapping()

		str := "testing 123"
		strToken := token(1)
		testMap.values[strToken] = &str

		gotStr := testMap.Get(strToken)
		assert.Equal(t, str, *gotStr.(*string))
	}
}
