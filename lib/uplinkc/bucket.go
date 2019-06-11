// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"context"
	"unsafe"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// CreateBucket creates a new bucket if authorized.
//export CreateBucket
func CreateBucket(projectHandle C.Project, name *C.char, bucketConfig *C.BucketConfig, cerr **C.char) C.BucketInfo {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketInfo{}
	}

	var config *uplink.BucketConfig
	if bucketConfig != nil {
		config = &uplink.BucketConfig{
			PathCipher: storj.CipherSuite(bucketConfig.path_cipher),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(bucketConfig.encryption_parameters.cipher_suite),
				BlockSize:   int32(bucketConfig.encryption_parameters.block_size),
			},
		}
		config.Volatile.RedundancyScheme = storj.RedundancyScheme{
			Algorithm: storj.RedundancyAlgorithm(bucketConfig.redundancy_scheme.algorithm),
			ShareSize: int32(bucketConfig.redundancy_scheme.share_size),
			RequiredShares: int16(bucketConfig.redundancy_scheme.required_shares),
			RepairShares: int16(bucketConfig.redundancy_scheme.repair_shares),
			OptimalShares: int16(bucketConfig.redundancy_scheme.optimal_shares),
			TotalShares: int16(bucketConfig.redundancy_scheme.total_shares),
		}
	}

	bucket, err := project.CreateBucket(project.scope.ctx, C.GoString(name), config)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketInfo{}
	}

	return newBucketInfo(bucket)
}

type Bucket struct {
	scope
	lib *libuplink.Bucket
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess information.
//export OpenBucket
func OpenBucket(projectHandle C.Project, name *C.char, caccess C.EncryptionAccess, cerr **C.char) C.Bucket {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.Bucket{}
	}

	var access uplink.EncryptionAccess
	copy(access.Key[:], caccess.key[:])

	scope := project.scope.child()

	bucket, err := project.lib.OpenBucket(scope.ctx, C.GoString(name), access)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Bucket{}
	}

	return C.Bucket{universe.Add(Bucket{scope, bucket})}
}

// CloseBucket closes a Bucket handle.
//export CloseBucket
func CloseBucket(bucketHandle C.Bucket) {
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

	bucketList, err := project.ListBuckets(project.scope.ctx, opts)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketList{}
	}

	listLen := len(bucketList.Items)
	infoSize := int(unsafe.Sizeof(C.BucketInfo{}))

	itemsPtr := C.malloc(uintptr(listLen * infoSize))
	items := (*[1<<32-1]C.BucketInfo)(unsafe.Pointer(itemsPtr))

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
	items := (*[1<<32-1]C.BucketInfo)(unsafe.Pointer(bucketlist.items))
	for i := 0; i < int(bucketlist.length); i++ {
		FreeBucketInfo(&items[0])
	}
	C.free(unsafe.Pointer(bucketlist.items))
	bucketlist.items = nil
}

// GetBucketInfo returns info about the requested bucket if authorized.
//export GetBucketInfo
func GetBucketInfo(cProject C.Project, bucketName *C.char, cerr **C.char) C.BucketInfo {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketInfo{}
	}

	bucket, _, err := project.GetBucketInfo(project.scope.ctx, C.GoString(bucketName))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketInfo{}
	}

	return newBucketInfo(bucket)
}
