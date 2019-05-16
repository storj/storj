// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"context"

	"storj.io/storj/lib/uplink"
)

//export CreateBucket
func CreateBucket(cProject C.GoUintptr, name *C.char, cCfg C.struct_BucketConfig, cErr **C.char) (cBucket C.struct_Bucket) {
	ctx := context.Background()
	project := (*uplink.Project)(goPointerFromCGoUintptr(cProject))

	cfg := new(uplink.BucketConfig)
	if err := CToGoStruct(cCfg, cfg); err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	bucket, err := project.CreateBucket(ctx, C.GoString(name), cfg)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	if err := CToGoStruct(cBucket, bucket); err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	return cBucket
}
