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

	encParamsPtr := CMalloc(unsafe.Sizeof(C.EncryptionParameters_t{}))
	encParams := (*C.EncryptionParameters_t)(unsafe.Pointer(encParamsPtr))
	*encParams = C.EncryptionParameters_t{
		cipher_suite: C.uint32_t(bucket.EncryptionParameters.CipherSuite),
		block_size:   C.int32_t(bucket.EncryptionParameters.BlockSize),
	}

	redundancySchemePtr := CMalloc(unsafe.Sizeof(C.RedundancyScheme_t{}))
	redundancyScheme := (*C.RedundancyScheme_t)(unsafe.Pointer(redundancySchemePtr))
	*redundancyScheme = C.RedundancyScheme_t{
		algorithm:       C.uint32_t(bucket.RedundancyScheme.Algorithm),
		share_size:      C.int32_t(bucket.RedundancyScheme.ShareSize),
		required_shares: C.int32_t(bucket.RedundancyScheme.RequiredShares),
		repair_shares:   C.int32_t(bucket.RedundancyScheme.RepairShares),
		optimal_shares:  C.int32_t(bucket.RedundancyScheme.OptimalShares),
		total_shares:    C.int32_t(bucket.RedundancyScheme.TotalShares),
	}

	return C.Bucket_t{
		encryption_parameters: encParams,
		redundancy_scheme:     redundancyScheme,
		name:                  C.CString(bucket.Name),
		created:               C.int64_t(bucket.Created.Unix()),
		path_cipher:           C.uint32_t(bucket.PathCipher),
		segment_size:          C.int64_t(bucket.SegmentsSize),
	}
}
