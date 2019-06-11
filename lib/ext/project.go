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
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// CreateBucket creates a new bucket if authorized.
//export CreateBucket
func CreateBucket(cProject C.ProjectRef_t, name *C.char, cBucketCfg *C.BucketConfig_t, cErr **C.char) (cBucket C.Bucket_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return cBucket
	}

	var bucketCfg *uplink.BucketConfig
	if unsafe.Pointer(cBucketCfg) != nil {
		bucketCfg = &uplink.BucketConfig{
			PathCipher: storj.CipherSuite(cBucketCfg.path_cipher),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(cBucketCfg.encryption_parameters.cipher_suite),
				BlockSize:   int32(cBucketCfg.encryption_parameters.block_size),
			},
		}
		bucketCfg.Volatile.RedundancyScheme = storj.RedundancyScheme{
			Algorithm: storj.RedundancyAlgorithm(cBucketCfg.redundancy_scheme.algorithm),
			ShareSize: int32(cBucketCfg.redundancy_scheme.share_size),
			RequiredShares: int16(cBucketCfg.redundancy_scheme.required_shares),
			RepairShares: int16(cBucketCfg.redundancy_scheme.repair_shares),
			OptimalShares: int16(cBucketCfg.redundancy_scheme.optimal_shares),
			TotalShares: int16(cBucketCfg.redundancy_scheme.total_shares),
		}
	}

	bucket, err := project.CreateBucket(ctx, C.GoString(name), bucketCfg)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	return NewCBucket(&bucket)
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
//export OpenBucket
func OpenBucket(cProject C.ProjectRef_t, name *C.char, cAccess *C.EncryptionAccess_t, cErr **C.char) (bucketRef C.BucketRef_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return bucketRef
	}

	var access *uplink.EncryptionAccess
	if unsafe.Pointer(cAccess) != nil {
		bytes := C.GoBytes(unsafe.Pointer(cAccess.key.bytes), cAccess.key.length)
		access = &uplink.EncryptionAccess{}
		copy(access.Key[:], bytes)
	}

	bucket, err := project.OpenBucket(ctx, C.GoString(name), access)
	if err != nil {
		*cErr = C.CString(err.Error())
		return bucketRef
	}

	return C.BucketRef_t(structRefMap.Add(bucket))
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
//export DeleteBucket
func DeleteBucket(cProject C.ProjectRef_t, bucketName *C.char, cErr **C.char) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return
	}

	if err := project.DeleteBucket(ctx, C.GoString(bucketName)); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

// ListBuckets will list authorized buckets.
//export ListBuckets
func ListBuckets(cProject C.ProjectRef_t, cOpts *C.BucketListOptions_t, cErr **C.char) (cBucketList C.BucketList_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return
	}

	var opts *uplink.BucketListOptions
	if cOpts != nil {
		opts = &uplink.BucketListOptions{
			Cursor:    C.GoString(cOpts.cursor),
			Direction: storj.ListDirection(cOpts.direction),
			Limit:     int(cOpts.limit),
		}
	}

	bucketList, err := project.ListBuckets(ctx, opts)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucketList
	}
	bucketListLen := len(bucketList.Items)

	bucketSize := int(unsafe.Sizeof(C.Bucket_t{}))
	cBucketsPtr := CMalloc(uintptr(bucketListLen * bucketSize))

	for i, bucket := range bucketList.Items {
		nextAddress := uintptr(int(cBucketsPtr) + (i * bucketSize))
		cBucket := (*C.Bucket_t)(unsafe.Pointer(nextAddress))
		*cBucket = NewCBucket(&bucket)
	}

	return C.BucketList_t{
		more:   C.bool(bucketList.More),
		items:  (*C.Bucket_t)(unsafe.Pointer(cBucketsPtr)),
		length: C.int32_t(bucketListLen),
	}
}

// GetBucketInfo returns info about the requested bucket if authorized.
//export GetBucketInfo
func GetBucketInfo(cProject C.ProjectRef_t, bucketName *C.char, cErr **C.char) (cBucketInfo C.BucketInfo_t) {
	ctx := context.Background()

	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return cBucketInfo
	}

	bucket, cfg, err := project.GetBucketInfo(ctx, C.GoString(bucketName))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucketInfo
	}

	return C.BucketInfo_t{
		bucket: NewCBucket(&bucket),
		config: C.BucketConfig_t{
			path_cipher:           C.uint8_t(cfg.PathCipher),
			encryption_parameters: NewCEncryptionParams(&cfg.EncryptionParameters),
		},
	}
}

// CloseProject closes the Project.
//export CloseProject
func CloseProject(cProject C.ProjectRef_t, cErr **C.char) {
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return
	}

	if err := project.Close(); err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	structRefMap.Del(token(cProject))
}
