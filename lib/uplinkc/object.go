// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"storj.io/storj/lib/uplink"
	"unsafe"
)

// Object is a scoped uplink.Object
type Object struct {
	scope
	*uplink.Object
}

// open_object returns an Object handle, if authorized.
//export open_object
func open_object(bucketHandle C.BucketRef_t, objectPath *C.char, cerr **C.char) C.ObjectRef_t {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return C.ObjectRef_t{}
	}

	scope := bucket.scope.child()

	object, err := bucket.OpenObject(scope.ctx, C.GoString(objectPath))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.ObjectRef_t{}
	}

	return C.ObjectRef_t{universe.Add(&Object{scope, object})}
}

// close_object closes the object.
//export close_object
func close_object(objectHandle C.ObjectRef_t, cerr **C.char) {
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

// get_object_meta returns the object meta which contains metadata about a specific Object.
//export get_object_meta
func get_object_meta(cObject C.ObjectRef_t, cErr **C.char) C.ObjectMeta_t {
	object, ok := universe.Get(cObject._handle).(*uplink.Object)
	if !ok {
		*cErr = C.CString("invalid object")
		return C.ObjectMeta_t{}
	}

	checksum, checksumLen := bytes_to_cbytes(object.Meta.Checksum, len(object.Meta.Checksum))

	mapRef := new_map_ref()
	for k, v := range object.Meta.Metadata {
		map_ref_set(mapRef, C.CString(k), C.CString(v), cErr)
		if C.GoString(*cErr) != "" {
			return C.ObjectMeta_t{}
		}
	}

	return C.ObjectMeta_t {
		bucket: C.CString(object.Meta.Bucket),
		path:  C.CString(object.Meta.Path),
		is_prefix: C.bool(object.Meta.IsPrefix),
		content_type:  C.CString(object.Meta.ContentType),
		meta_data: mapRef,
		created: C.time_t(object.Meta.Created.UnixNano()),
		modified: C.time_t(object.Meta.Modified.UnixNano()),
		expires: C.time_t(object.Meta.Expires.UnixNano()),
		size: C.uint64_t(object.Meta.Size),
		checksum_bytes: checksum,
		checksum_length: checksumLen,
	}
}

// free_object_meta frees the object meta
//export free_object_meta
func free_object_meta(objectMeta *C.ObjectMeta_t) {
	C.free(unsafe.Pointer(objectMeta.bucket))
	objectMeta.bucket = nil

	C.free(unsafe.Pointer(objectMeta.path))
	objectMeta.path = nil

	C.free(unsafe.Pointer(objectMeta.content_type))
	objectMeta.content_type = nil

	C.free(unsafe.Pointer(objectMeta.checksum_bytes))
	objectMeta.checksum_bytes = nil

	universe.Del(objectMeta.meta_data._handle)
}

//// download_range returns an Object's data. A length of -1 will mean
//// (Object.Size - offset).
////export download_range
//func download_range(objectRef C.ObjectRef_t, offset C.int64_t, length C.int64_t, file *File, cErr **C.char) {
//	object, ok := universe.Get(objectRef._handle).(*Object)
//	if !ok {
//		*cErr = C.CString("invalid object")
//		return
//	}
//
//	scope := object.scope.child()
//
//	rc, err := object.DownloadRange(scope.ctx, int64(offset), int64(length))
//	if err != nil {
//		*cErr = C.CString(err.Error())
//		return
//	}
//
//	defer rc.Close()
//	_, err = io.Copy(file, rc)
//	if err != io.EOF && err != nil {
//		*cErr = C.CString(err.Error())
//		return
//	}
//}