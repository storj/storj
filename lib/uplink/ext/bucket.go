// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import "storj.io/storj/lib/uplink"

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
