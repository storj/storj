// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "C"
import (
	"context"
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

//export CreateBucket
func CreateBucket(cProject CProjectRef, name CCharPtr, cBucketCfg *CBucketConfig, cErr *CCharPtr) (cBucket CBucket) {
	ctx := context.Background()
	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
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

	bucket, err := project.CreateBucket(ctx, CGoString(name), bucketCfg)
	if err != nil {
		*cErr = CCString(err.Error())
		return cBucket
	}

	return NewCBucket(&bucket)
}

//export OpenBucket
func OpenBucket(cProject CProjectRef, name CCharPtr, cAccess *CEncryptionAccess, cErr *CCharPtr) (bucketRef CBucketRef) {
	ctx := context.Background()
	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
		return bucketRef
	}

	var access *uplink.EncryptionAccess
	if unsafe.Pointer(cAccess) != nil {
		bytes := CGoBytes(unsafe.Pointer(cAccess.key.bytes), cAccess.key.length)
		access = &uplink.EncryptionAccess{}
		copy(access.Key[:], bytes)
	}

	bucket, err := project.OpenBucket(ctx, CGoString(name), access)
	if err != nil {
		*cErr = CCString(err.Error())
		return bucketRef
	}

	return CBucketRef(universe.Add(bucket))
}

//export DeleteBucket
func DeleteBucket(cProject CProjectRef, bucketName CCharPtr, cErr *CCharPtr) {
	ctx := context.Background()
	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
		return
	}

	if err := project.DeleteBucket(ctx, CGoString(bucketName)); err != nil {
		*cErr = CCString(err.Error())
		return
	}
}

//export ListBuckets
func ListBuckets(cProject CProjectRef, cOpts *CBucketListOptions, cErr *CCharPtr) (cBucketList CBucketList) {
	ctx := context.Background()
	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
		return
	}

	var opts *uplink.BucketListOptions
	if cOpts != nil {
		opts = &uplink.BucketListOptions{
			Cursor:    CGoString(cOpts.cursor),
			Direction: storj.ListDirection(cOpts.direction),
			Limit:     int(cOpts.limit),
		}
	}

	bucketList, err := project.ListBuckets(ctx, opts)
	if err != nil {
		*cErr = CCString(err.Error())
		return cBucketList
	}
	bucketListLen := len(bucketList.Items)

	bucketSize := int(unsafe.Sizeof(CBucket{}))
	// TODO: use `calloc` instead?
	cBucketsPtr := GoCMalloc(uintptr(bucketListLen * bucketSize))

	for i, bucket := range bucketList.Items {
		nextAddress := uintptr(int(cBucketsPtr) + (i * bucketSize))
		cBucket := (*CBucket)(unsafe.Pointer(nextAddress))
		*cBucket = NewCBucket(&bucket)
	}

	return CBucketList{
		more:   CBool(bucketList.More),
		items:  (*CBucket)(unsafe.Pointer(cBucketsPtr)),
		length: CInt32(bucketListLen),
	}
}

//export GetBucketInfo
func GetBucketInfo(cProject CProjectRef, bucketName CCharPtr, cErr *CCharPtr) (cBucketInfo CBucketInfo) {
	ctx := context.Background()

	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
		return cBucketInfo
	}

	bucket, cfg, err := project.GetBucketInfo(ctx, CGoString(bucketName))
	if err != nil {
		*cErr = CCString(err.Error())
		return cBucketInfo
	}

	return CBucketInfo{
		bucket: NewCBucket(&bucket),
		config: CBucketConfig{
			path_cipher:           CUint8(cfg.PathCipher),
			encryption_parameters: NewCEncryptionParamsPtr(&cfg.EncryptionParameters),
		},
	}
}

//export CloseProject
func CloseProject(cProject CProjectRef, cErr *CCharPtr) {
	project, ok := universe.Get(Token(cProject)).(*uplink.Project)
	if !ok {
		*cErr = CCString("invalid project")
		return
	}

	if err := project.Close(); err != nil {
		*cErr = CCString(err.Error())
		return
	}

	universe.Del(Token(cProject))
}
