// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"context"
	"io"
	"storj.io/storj/lib/uplink"
	"unsafe"
)

//export CloseObject
func CloseObject(cObject C.ObjectRef_t, cErr **C.char) {
	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return
	}

	if err := object.Close(); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export DownloadRange
func DownloadRange(cObject C.ObjectRef_t, offset C.int64_t, length C.int64_t, callback uintptr, cErr **C.char) {
	ctx := context.Background()

	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}

	rc, err := object.DownloadRange(ctx, int64(offset), int64(length))
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}
	defer rc.Close()

	// TODO: This size could be optimized
	buf := make([]byte, 1024)

	for {
		n, err := rc.Read(buf)
		if err == io.EOF {
			callback(nil, C.bool(true))
			break
		}

		ptr := CMalloc(uintptr(n))
		mem := unsafe.Pointer(ptr)
		for i := 0; i < n; i++ {
			nextAddress := uintptr(int(ptr) + i)
			*(*uint8)(unsafe.Pointer(nextAddress)) = buf[i]
		}

		bytes := C.Bytes_t{
			length: C.int32_t(n),
			bytes: (*C.uint8_t)(mem),
		}

		callback(bytes, C.bool(false))
	}
}