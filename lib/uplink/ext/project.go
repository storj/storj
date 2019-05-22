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

	"storj.io/storj/lib/uplink"
)

//export CreateBucket
func CreateBucket(cProject C.ProjectRef, name *C.char, bucketConfigRef C.BucketConfigRef, cErr **C.char) (cBucket C.BucketRef) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return cBucket
	}

	bucketCfg, ok := structRefMap.Get(token(bucketConfigRef)).(*uplink.BucketConfig)

	bucket, err := project.CreateBucket(ctx, C.GoString(name), bucketCfg)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	return C.GoUintptr(structRefMap.Add(&bucket))
}
