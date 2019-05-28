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
	"storj.io/storj/pkg/storj"
	"unsafe"
)

//export CreateBucket
func CreateBucket(cProject C.ProjectRef_t, name *C.char, cBucketCfg C.BucketConfig_t, cErr **C.char) (cBucket C.Bucket_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return cBucket
	}

	bucketCfg := uplink.BucketConfig{
		PathCipher: storj.CipherSuite(cBucketCfg.path_cipher),
		EncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(cBucketCfg.encryption_parameters.cipher_suite),
			BlockSize:   int32(cBucketCfg.encryption_parameters.block_size),
		},
	}

	bucket, err := project.CreateBucket(ctx, C.GoString(name), &bucketCfg)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cBucket
	}

	return bucketToCBucket(&bucket)
}

func OpenBucket(cProject C.ProjectRef_t, name *C.char, cAccess *C.EncryptionAccess_t, cErr **C.char) (bucketRef C.BucketRef_t) {
	ctx := context.Background()
	project, ok := structRefMap.Get(token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = C.CString("invalid project")
		return bucketRef
	}

	var access uplink.EncryptionAccess
	bytes := C.GoBytes(unsafe.Pointer(cAccess.Key.bytes), cAccess.Key.length)
	copy(access.Key[:], bytes)

	bucket, err := project.OpenBucket(ctx, C.GoString(name), &access)
	if err != nil {
		*cErr = C.CString(err.Error())
		return bucketRef
	}

	return C.BucketRef_t(structRefMap.Add(bucket))
}

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
	cBucketsPtr := CMalloc(uintptr((bucketListLen - 1) * bucketSize))

	for i, bucket := range bucketList.Items {
		cBucket := (*C.Bucket_t)(unsafe.Pointer(uintptr(int(cBucketsPtr) + (i * bucketSize))))
		*cBucket = bucketToCBucket(&bucket)
	}

	return C.BucketList_t{
		more:   C.bool(bucketList.More),
		items:  (*C.Bucket_t)(unsafe.Pointer(cBucketsPtr)),
		length: C.int32_t(bucketListLen),
	}
}

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
}

func bucketToCBucket(bucket *storj.Bucket) C.Bucket_t {
	encParamsPtr := CMalloc(unsafe.Sizeof(C.EncryptionParameters_t{}))
	encParams := (*C.EncryptionParameters_t)(unsafe.Pointer(encParamsPtr))
	*encParams = C.EncryptionParameters_t{
		cipher_suite: C.uint8_t(bucket.EncryptionParameters.CipherSuite),
		block_size:   C.int32_t(bucket.EncryptionParameters.BlockSize),
	}

	redundancySchemePtr := CMalloc(unsafe.Sizeof(C.RedundancyScheme_t{}))
	redundancyScheme := (*C.RedundancyScheme_t)(unsafe.Pointer(redundancySchemePtr))
	*redundancyScheme = C.RedundancyScheme_t{
		algorithm:       C.uint8_t(bucket.RedundancyScheme.Algorithm),
		share_size:      C.int32_t(bucket.RedundancyScheme.ShareSize),
		required_shares: C.int16_t(bucket.RedundancyScheme.RequiredShares),
		repair_shares:   C.int16_t(bucket.RedundancyScheme.RepairShares),
		optimal_shares:  C.int16_t(bucket.RedundancyScheme.OptimalShares),
		total_shares:    C.int16_t(bucket.RedundancyScheme.TotalShares),
	}

	return C.Bucket_t{
		encryption_parameters: encParams,
		redundancy_scheme:     redundancyScheme,
		name:                  C.CString(bucket.Name),
		created:               C.int64_t(bucket.Created.Unix()),
		path_cipher:           C.uint8_t(bucket.PathCipher),
		segment_size:          C.int64_t(bucket.SegmentsSize),
	}
}
