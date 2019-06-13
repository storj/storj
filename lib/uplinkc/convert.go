// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"unsafe"
)

// newObjectInfo returns a C object struct converted from a go object struct.
func newObjectInfo (object *storj.Object) C.ObjectInfo_t {
	return C.ObjectInfo_t {
		version:      C.uint32_t(object.Version),
		bucket:       newBucketInfo(&object.Bucket),
		path:         C.CString(object.Path),
		is_prefix:    C.bool(object.IsPrefix),
		metadata:     C.MapRef_t{universe.Add(object.Metadata)},
		content_type: C.CString(object.ContentType),
		created: C.time_t(object.Created.Unix()),
		modified: C.time_t(object.Modified.Unix()),
		expires: C.time_t(object.Expires.Unix()),
	}
}

// newBucketInfo returns a C bucket struct converted from a go bucket struct.
func newBucketInfo(bucket *storj.Bucket) C.BucketInfo_t {
	return C.BucketInfo_t{
		name:         C.CString(bucket.Name),
		created:      C.time_t(bucket.Created.Unix()),
		path_cipher:  C.uint8_t(bucket.PathCipher),
		segment_size: C.uint64_t(bucket.SegmentsSize),

		encryption_parameters: convertEncryptionParameters(&bucket.EncryptionParameters),
		redundancy_scheme:     convertRedundancyScheme(&bucket.RedundancyScheme),
	}
}

// convertEncryptionParameters converts Go EncryptionParameters to C.
func convertEncryptionParameters(goParams *storj.EncryptionParameters) C.EncryptionParameters_t {
	return C.EncryptionParameters_t{
		cipher_suite: C.uint8_t(goParams.CipherSuite),
		block_size:   C.int32_t(goParams.BlockSize),
	}
}

// convertRedundancyScheme converts Go RedundancyScheme to C.
func convertRedundancyScheme(scheme *storj.RedundancyScheme) C.RedundancyScheme_t {
	return C.RedundancyScheme_t{
		algorithm:       C.uint8_t(scheme.Algorithm),
		share_size:      C.int32_t(scheme.ShareSize),
		required_shares: C.int16_t(scheme.RequiredShares),
		repair_shares:   C.int16_t(scheme.RepairShares),
		optimal_shares:  C.int16_t(scheme.OptimalShares),
		total_shares:    C.int16_t(scheme.TotalShares),
	}
}

// newBucketConfig returns a C bucket config struct converted from a go bucket config struct.
func newBucketConfig(bucketCfg *uplink.BucketConfig) C.BucketConfig_t {
	return C.BucketConfig_t{
		encryption_parameters: convertEncryptionParameters(&bucketCfg.EncryptionParameters),
		redundancy_scheme:     convertRedundancyScheme(&bucketCfg.Volatile.RedundancyScheme),
		path_cipher:           C.uint8_t(bucketCfg.PathCipher),
	}
}

// bytes_to_cbytes converts a byte array to a C uint8_t array
func bytes_to_cbytes(bytes []byte, lenOfBytes int) (data *C.uint8_t, len C.uint64_t) {
	ptr := uintptr(C.malloc(C.size_t(lenOfBytes)))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < lenOfBytes; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = bytes[i]
	}

	len = C.uint64_t(lenOfBytes)
	data = (*C.uint8_t)(mem)

	return data, len
}