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
	"fmt"
	"io"
	"os"
	"storj.io/storj/lib/uplink"
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
func DownloadRange(cObject C.ObjectRef_t, offset C.int64_t, length C.int64_t, path *C.char, cErr **C.char) {
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

	if _, err := os.Stat(C.GoString(path)); os.IsExist(err) {
		*cErr = C.CString(fmt.Sprintf("Path (%s) already exists", C.GoString(path)))
		return
	}

	f, err := os.OpenFile(C.GoString(path), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	if _, err := io.Copy(f, rc); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}