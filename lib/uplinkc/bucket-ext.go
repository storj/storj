// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
// #include <stdio.h>
import "C"
import (
	"context"
	"storj.io/storj/pkg/storj"
	"time"
	"unsafe"

	"storj.io/storj/lib/uplink"
)

// UploadObject uploads a new object, if authorized.
//export UploadObject
func UploadObject(cBucket C.BucketRef_t, path *C.char, reader *File, cOpts *C.UploadOptions_t, cErr **C.char) {
	ctx := context.Background()

	bucket, ok := structRefMap.Get(token(cBucket)).(*uplink.Bucket)
	if !ok {
		*cErr = C.CString("invalid bucket")
		return
	}

	var opts *uplink.UploadOptions
	if cOpts != nil {
		var metadata map[string]string
		if uintptr(cOpts.metadata) != 0 {
			metadata, ok = structRefMap.Get(token(cOpts.metadata)).(map[string]string)
			if !ok {
				*cErr = C.CString("invalid metadata in upload options")
				return
			}
		}

		opts = &uplink.UploadOptions{
			ContentType: C.GoString(cOpts.content_type),
			Metadata:    metadata,
			Expires:     time.Unix(int64(cOpts.expires), 0),
		}
	}

	if err := bucket.UploadObject(ctx, C.GoString(path), reader, opts); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

// ListObjects lists objects a user is authorized to see.
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

	objectSize := int(unsafe.Sizeof(C.ObjectRef_t{}))
	cObjectsPtr := CMalloc(uintptr(objListLen * objectSize))

	for i, object := range objectList.Items {
		nextAddress := uintptr(int(cObjectsPtr) + (i * objectSize))
		cObject := (*C.ObjectInfo_t)(unsafe.Pointer(nextAddress))
		*cObject = newObjectInfo(&object)
	}

	return C.ObjectList_t{
		bucket: C.CString(objectList.Bucket),
		prefix: C.CString(objectList.Prefix),
		more: C.bool(objectList.More),
		items:  (*C.ObjectInfo_t)(unsafe.Pointer(cObjectsPtr)),
		length: C.int32_t(objListLen),
	}
}