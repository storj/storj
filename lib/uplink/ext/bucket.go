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
	"time"

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
