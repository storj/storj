// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"

	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
)

// Bucket is a scoped uplink.Bucket
type Bucket struct {
	scope
	*uplink.Bucket
}

//export create_bucket
// create_bucket creates a new bucket if authorized.
func create_bucket(projectHandle C.ProjectRef, name *C.char, bucketConfig *C.BucketConfig, cerr **C.char) C.BucketInfo {
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
			Algorithm:      storj.RedundancyAlgorithm(bucketConfig.redundancy_scheme.algorithm),
			ShareSize:      int32(bucketConfig.redundancy_scheme.share_size),
			RequiredShares: int16(bucketConfig.redundancy_scheme.required_shares),
			RepairShares:   int16(bucketConfig.redundancy_scheme.repair_shares),
			OptimalShares:  int16(bucketConfig.redundancy_scheme.optimal_shares),
			TotalShares:    int16(bucketConfig.redundancy_scheme.total_shares),
		}
	}

	bucket, err := project.CreateBucket(project.scope.ctx, C.GoString(name), config)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.BucketInfo{}
	}

	return newBucketInfo(&bucket)
}

//export get_bucket_info
// get_bucket_info returns info about the requested bucket if authorized.
func get_bucket_info(projectHandle C.ProjectRef, bucketName *C.char, cerr **C.char) C.BucketInfo {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketInfo{}
	}

	bucket, _, err := project.GetBucketInfo(project.scope.ctx, C.GoString(bucketName))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.BucketInfo{}
	}

	return newBucketInfo(&bucket)
}

//export open_bucket
// open_bucket returns a Bucket handle with the given encryption context information.
func open_bucket(projectHandle C.ProjectRef, name *C.char, encryptionAccess *C.char, cerr **C.char) C.BucketRef {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketRef{}
	}

	access, err := uplink.ParseEncryptionAccess(C.GoString(encryptionAccess))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.BucketRef{}
	}

	scope := project.scope.child()

	bucket, err := project.OpenBucket(scope.ctx, C.GoString(name), access)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.BucketRef{}
	}

	return C.BucketRef{universe.Add(&Bucket{scope, bucket})}
}

//export list_buckets
// list_buckets will list authorized buckets.
func list_buckets(projectHandle C.ProjectRef, bucketListOptions *C.BucketListOptions, cerr **C.char) C.BucketList {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketList{}
	}

	var opts *uplink.BucketListOptions
	if bucketListOptions != nil {
		opts = &uplink.BucketListOptions{
			Cursor:    C.GoString(bucketListOptions.cursor),
			Direction: storj.Forward,
			Limit:     int(bucketListOptions.limit),
		}
	}

	bucketList, err := project.ListBuckets(project.scope.ctx, opts)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.BucketList{}
	}

	listLen := len(bucketList.Items)
	infoSize := int(unsafe.Sizeof(C.BucketInfo{}))

	itemsPtr := C.malloc(C.size_t(listLen * infoSize))
	items := *(*[]C.BucketInfo)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(itemsPtr),
			Len:  listLen,
			Cap:  listLen,
		},
	))
	for i, bucket := range bucketList.Items {
		bucket := bucket
		items[i] = newBucketInfo(&bucket)
	}

	return C.BucketList{
		more:   C.bool(bucketList.More),
		items:  &items[0],
		length: C.int32_t(listLen),
	}
}

//export delete_bucket
// delete_bucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
func delete_bucket(projectHandle C.ProjectRef, bucketName *C.char, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return
	}

	if err := project.DeleteBucket(project.scope.ctx, C.GoString(bucketName)); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}

//export close_bucket
// close_bucket closes a Bucket handle.
func close_bucket(bucketHandle C.BucketRef, cerr **C.char) {
	bucket, ok := universe.Get(bucketHandle._handle).(*Bucket)
	if !ok {
		*cerr = C.CString("invalid bucket")
		return
	}

	universe.Del(bucketHandle._handle)
	defer bucket.cancel()

	if err := bucket.Close(); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}

//export free_bucket_info
// free_bucket_info frees bucket info.
func free_bucket_info(bucketInfo *C.BucketInfo) {
	C.free(unsafe.Pointer(bucketInfo.name))
	bucketInfo.name = nil
}

//export free_bucket_list
// free_bucket_list will free a list of buckets
func free_bucket_list(bucketlist *C.BucketList) {
	items := *(*[]C.BucketInfo)(unsafe.Pointer(
		&reflect.SliceHeader{
			Data: uintptr(unsafe.Pointer(bucketlist.items)),
			Len:  int(bucketlist.length),
			Cap:  int(bucketlist.length),
		},
	))
	for i := range items {
		free_bucket_info(&items[i])
	}
	C.free(unsafe.Pointer(bucketlist.items))
	bucketlist.items = nil
	bucketlist.length = 0
}
