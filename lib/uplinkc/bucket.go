// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// CloseBucket closes a Bucket handle.
//export CloseBucket
func CloseBucket(bucketHandle C.Bucket, cerr **C.char) {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return
	}

	universe.Del(bucketHandle._handle)
	defer bucket.cancel()

	if err := bucket.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
//export DeleteBucket
func DeleteBucket(projectHandle C.Project, bucketName *C.char, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return
	}

	if err := project.lib.DeleteBucket(project.scope.ctx, C.GoString(bucketName)); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

// ListBuckets will list authorized buckets.
//export ListBuckets
func ListBuckets(projectHandle C.Project, bucketListOptions *C.BucketListOptions, cerr **C.char) C.BucketList {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketList{}
	}

	var opts *uplink.BucketListOptions
	if bucketListOptions != nil {
		opts = &uplink.BucketListOptions{
			Cursor:    C.GoString(bucketListOptions.cursor),
			Direction: storj.ListDirection(bucketListOptions.direction),
			Limit:     int(bucketListOptions.limit),
		}
	}

	bucketList, err := project.lib.ListBuckets(project.scope.ctx, opts)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketList{}
	}

	listLen := len(bucketList.Items)
	infoSize := int(unsafe.Sizeof(C.BucketInfo{}))

	itemsPtr := C.malloc(C.size_t(listLen * infoSize))
	items := (*[1<<30]C.BucketInfo)(unsafe.Pointer(itemsPtr))
	for i, bucket := range bucketList.Items {
		items[i] = newBucketInfo(&bucket)
	}

	return C.BucketList{
		more:   C.bool(bucketList.More),
		items:  &items[0],
		length: C.int32_t(listLen),
	}
}

// FreeBucketList will free a list of buckets
//export FreeBucketList
func FreeBucketList(bucketlist *C.BucketList) {
	items := (*[1<<30]C.BucketInfo)(unsafe.Pointer(bucketlist.items))
	for i := 0; i < int(bucketlist.length); i++ {
		FreeBucketInfo(&items[0])
	}
	C.free(unsafe.Pointer(bucketlist.items))
	bucketlist.items = nil
}

// GetBucketInfo returns info about the requested bucket if authorized.
//export GetBucketInfo
func GetBucketInfo(projectHandle C.Project, bucketName *C.char, cerr **C.char) C.BucketInfo {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketInfo{}
	}

	bucket, _, err := project.lib.GetBucketInfo(project.scope.ctx, C.GoString(bucketName))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketInfo{}
	}

	return newBucketInfo(&bucket)
}
