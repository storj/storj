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
	"storj.io/storj/internal/readcloser"
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
func DownloadRange(cObject C.ObjectRef_t, offset C.int64_t, length C.int64_t, cErr **C.char) (downloader C.DownloadReaderRef_t) {
	ctx := context.Background()

	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return downloader
	}

	rc, err := object.DownloadRange(ctx, int64(offset), int64(length))
	if err != nil {
		*cErr = C.CString(err.Error())
		return downloader
	}

	return C.DownloadReaderRef_t(structRefMap.Add(rc))
}

//export Download
func Download(downloader C.DownloadReaderRef_t, bytes *C.Bytes_t, cErr **C.char) (readLength C.int){
	readCloser, ok := structRefMap.Get(token(downloader)).(*readcloser.LimitedReadCloser)
	if !ok {
		*cErr = C.CString("invalid reader")
		return C.int(0)
	}

	// TODO: This size could be optimized
	buf := make([]byte, 1024)

	n, err := readCloser.Read(buf)
	if err == io.EOF {
		readCloser.Close()
		return C.EOF
	}

	ptr := CMalloc(uintptr(n))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < n; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = buf[i]
	}

	bytes.length = C.int32_t(n)
	bytes.bytes = (*C.uint8_t)(mem)

	return C.int(n)
}

//export ObjectMeta
func ObjectMeta(cObject C.ObjectRef_t, cErr **C.char) (objectMeta C.ObjectMeta_t) {
	object, ok := structRefMap.Get(token(cObject)).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return objectMeta
	}

	checksumLen := len(object.Meta.Checksum)
	ptr := CMalloc(uintptr(checksumLen))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < checksumLen; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = object.Meta.Checksum[i]
	}

	bytes := C.Bytes_t{
		length: C.int32_t(checksumLen),
		bytes: (*C.uint8_t)(mem),
	}

	return C.ObjectMeta_t {
		Bucket: C.CString(object.Meta.Bucket),
		Path:  C.CString(object.Meta.Path),
		IsPrefix: C.bool(object.Meta.IsPrefix),
		ContentType:  C.CString(object.Meta.ContentType),
		// TODO: Metadata
		Created: C.uint64_t(object.Meta.Created.Unix()),
		Modified: C.uint64_t(object.Meta.Modified.Unix()),
		Expires: C.uint64_t(object.Meta.Expires.Unix()),
		Size: C.uint64_t(object.Meta.Size),
		Checksum: bytes,
	}
}
