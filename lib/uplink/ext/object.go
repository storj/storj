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

	structRefMap.Del(token(cObject))
}

//export DownloadRange
func DownloadRange(cObject C.ObjectRef_t, offset C.int64_t, length C.int64_t, file *File, cErr **C.char) {
	ctx := context.Background()

	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return
	}

	rc, err := object.DownloadRange(ctx, int64(offset), int64(length))
	if err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	defer rc.Close()
	_, err = io.Copy(file, rc)
	if err != io.EOF && err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export ObjectMeta
func ObjectMeta(cObject C.ObjectRef_t, cErr **C.char) (objectMeta C.ObjectMeta_t) {
	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return objectMeta
	}

	bytes := new(C.Bytes_t)
	bytesToCbytes(object.Meta.Checksum, len(object.Meta.Checksum), bytes)

	return C.ObjectMeta_t {
		Bucket: C.CString(object.Meta.Bucket),
		Path:  C.CString(object.Meta.Path),
		IsPrefix: C.bool(object.Meta.IsPrefix),
		ContentType:  C.CString(object.Meta.ContentType),
		MetaData: NewMapRef(),
		Created: C.uint64_t(object.Meta.Created.Unix()),
		Modified: C.uint64_t(object.Meta.Modified.Unix()),
		Expires: C.uint64_t(object.Meta.Expires.Unix()),
		Size: C.uint64_t(object.Meta.Size),
		Checksum: *bytes,
	}
}
