// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"storj.io/storj/lib/uplink"
)

// Object is a scoped uplink.Object
type Object struct {
	scope
	*uplink.Object
}

// open_object returns an Object handle, if authorized.
//export open_object
func open_object(bucketHandle C.Bucket, objectPath *C.char, cerr **C.char) C.Object {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return C.Object{}
	}

	scope := bucket.scope.child()

	object, err := bucket.OpenObject(scope.ctx, C.GoString(objectPath))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Object{}
	}

	return C.Object{universe.Add(&Object{scope, object})}
}

// close_object closes the object.
//export close_object
func close_object(objectHandle C.Object, cerr **C.char) {
	object, ok := universe.Get(objectHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid object")
		return
	}

	universe.Del(objectHandle._handle)
	defer object.cancel()

	if err := object.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}