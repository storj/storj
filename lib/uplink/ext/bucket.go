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
	"bytes"
	"context"
	"storj.io/storj/pkg/storj"
	"time"
	"unsafe"

	"storj.io/storj/lib/uplink"
)

//export UploadObject
func UploadObject(cBucket C.BucketRef_t, path *C.char, dataRef C.BufferRef_t, cOpts *C.UploadOptions_t, cErr **C.char) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(token(cBucket)).(*uplink.Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}

	// TODO: should `unsafe.Pointer(cOpts) == nil` be an error?
	var metadata map[string]string
	if uintptr(cOpts.metadata) != 0 {
		metadata, ok = structRefMap.Get(token(cOpts.metadata)).(map[string]string)
		if !ok {
			*cErr = C.CString("invalid metadata in upload options")
			return
		}
	}

	data, ok := structRefMap.Get(token(dataRef)).(*bytes.Buffer)
	if !ok {
		*cErr = C.CString("invalid data")
		return
	}

	var opts *uplink.UploadOptions
	if cOpts != nil {
		opts = &uplink.UploadOptions{
			ContentType: C.GoString(cOpts.content_type),
			Metadata:    metadata,
			Expires:     time.Unix(int64(cOpts.expires), 0),
		}
	}

	if err := bucket.UploadObject(ctx, C.GoString(path), data, opts); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export ListObjects
func ListObjects(bucketRef C.BucketRef_t, cListOpts *C.ListOptions_t, cErr **C.char) (cObjList C.ObjectList_t) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(token(bucketRef)).(*uplink.Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return cObjList
	}

	var opts *uplink.ListOptions
	if unsafe.Pointer(cListOpts) != nil {
		opts = &uplink.ListOptions{
			Prefix: C.GoString(cListOpts.cursor),
			Cursor: C.GoString(cListOpts.cursor),
			Delimiter: rune(cListOpts.delimiter),
			Recursive: bool(cListOpts.recursive),
			Direction: storj.ListDirection(cListOpts.direction),
			Limit: int(cListOpts.limit),
		}
	}

	objectList, err := bucket.ListObjects(ctx, opts)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cObjList
	}
	objListLen := len(objectList.Items)

	objectSize := int(unsafe.Sizeof(C.Object_t{}))
	// TODO: use `calloc` instead?
	cObjectsPtr := CMalloc(uintptr((objListLen - 1) * objectSize))

	for i, object := range objectList.Items {
		cObject := (*C.Object_t)(unsafe.Pointer(uintptr(int(cObjectsPtr) + (i * objectSize))))
		*cObject = NewCObject(&object)
	}

	return C.ObjectList_t{
		bucket: C.CString(objectList.Bucket),
		prefix: C.CString(objectList.Prefix),
		more: C.bool(objectList.More),
		items:  (*C.Object_t)(unsafe.Pointer(cObjectsPtr)),
		length: C.int32_t(objListLen),
	}
}

//export CloseBucket
func CloseBucket(bucketRef C.BucketRef_t, cErr **C.char) {
	bucket, ok := structRefMap.Get(token(bucketRef)).(*uplink.Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}


	if err := bucket.Close(); err != nil {
		*cErr = C.CString(err.Error())
		return
	}

}

func NewCObject(object *storj.Object) C.Object_t {
	return C.Object_t {
		version:      C.uint32_t(object.Version),
		bucket:       NewCBucket(&object.Bucket),
		path:         C.CString(object.Path),
		is_prefix:    C.bool(object.IsPrefix),
		metadata:     C.MapRef_t(structRefMap.Add(object.Metadata)),
		content_type: C.CString(object.ContentType),
		// TODO: use `UnixNano()`?
		created: C.time_t(object.Created.Unix()),
		modified: C.time_t(object.Modified.Unix()),
		expires: C.time_t(object.Expires.Unix()),
	}
}
