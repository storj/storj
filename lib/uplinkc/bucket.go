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
		return cBucket
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
		return cBucket
	}

	return &C.BucketInfo{
		name: C.CString(bucket.Name),

		created:      C.int64_t(bucket.Created.Unix()),
		path_cipher:  C.uint8_t(bucket.PathCipher),
		segment_size: C.int64_t(bucket.SegmentsSize),

		encryption_parameters: C.EncryptionParameters{
			cipher_suite: uint8_t(bucket.EncryptionParameters.CipherSuite),
			block_size: int32_t(bucket.EncryptionParameters.BlockSize),
		},
		redundancy_scheme: C.RedundancyScheme{
			algorithm: C.uint8_t(bucket.RedundancyScheme.Algorithm)
			share_size: C.int32_t(bucket.RedundancyScheme.ShareSize)
			required_shares: C.uint16_t(bucket.RedundancyScheme.RequiredShares)
			repair_shares: C.uint16_t(bucket.RedundancyScheme.RepairShares)
			optimal_shares: C.uint16_t(bucket.RedundancyScheme.OptimalShares)
			total_shares: C.uint16_t(bucket.RedundancyScheme.TotalShares)
		},
	}
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
//export OpenBucket
func OpenBucket(projectHandle C.Project, name *C.char, cAccess *C.EncryptionAccess, cerr **C.char) (bucketRef C.Bucket) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return cBucket
	}

	var access *uplink.EncryptionAccess
	if cAccess != nil {
		access.Key
		bytes := C.GoBytes(unsafe.Pointer(cAccess.key.bytes), cAccess.key.length)
		access = &uplink.EncryptionAccess{}
		copy(access.Key[:], bytes)
	}

	bucket, err := project.OpenBucket(ctx, C.GoString(name), access)
	if err != nil {
		*cerr = C.CString(err.Error())
		return bucketRef
	}

	return C.Bucket(structRefMap.Add(bucket))
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
//export DeleteBucket
func DeleteBucket(cProject C.Project, bucketName *C.char, cerr **C.char) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return
	}

	if err := project.DeleteBucket(ctx, C.GoString(bucketName)); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

// ListBuckets will list authorized buckets.
//export ListBuckets
func ListBuckets(cProject C.Project, cOpts *C.BucketListOptions_t, cerr **C.char) (cBucketList C.BucketList_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cerr = C.CString("invalid project")
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
		*cerr = C.CString(err.Error())
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
func GetBucketInfo(cProject C.Project, bucketName *C.char, cerr **C.char) (cBucketInfo C.BucketInfo_t) {
	ctx := context.Background()

	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return cBucketInfo
	}

	bucket, cfg, err := project.GetBucketInfo(ctx, C.GoString(bucketName))
	if err != nil {
		*cerr = C.CString(err.Error())
		return cBucketInfo
	}

	return C.BucketInfo_t{
		bucket: NewCBucket(&bucket),
		config: C.BucketConfig{
			path_cipher:           C.uint8_t(cfg.PathCipher),
			encryption_parameters: NewCEncryptionParams(&cfg.EncryptionParameters),
		},
	}
}
