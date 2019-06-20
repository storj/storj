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

// newBucketConfig returns a C bucket config struct converted from a go bucket config struct.
func newBucketConfig(bucketCfg *uplink.BucketConfig) C.BucketConfig {
	return C.BucketConfig{
		encryption_parameters: convertEncryptionParameters(&bucketCfg.EncryptionParameters),
		redundancy_scheme:     convertRedundancyScheme(&bucketCfg.Volatile.RedundancyScheme),
		path_cipher:           C.uint8_t(bucketCfg.PathCipher),
	}
}

// newBucketInfo returns a C bucket struct converted from a go bucket struct.
func newBucketInfo(bucket *storj.Bucket) C.BucketInfo {
	return C.BucketInfo{
		name:         C.CString(bucket.Name),
		created:      C.int64_t(bucket.Created.Unix()),
		path_cipher:  C.uint8_t(bucket.PathCipher),
		segment_size: C.uint64_t(bucket.SegmentsSize),

		encryption_parameters: convertEncryptionParameters(&bucket.EncryptionParameters),
		redundancy_scheme:     convertRedundancyScheme(&bucket.RedundancyScheme),
	}
}

// newObjectInfo returns a C object struct converted from a go object struct.
func newObjectInfo(object *storj.Object) C.ObjectInfo {
	return C.ObjectInfo{
		version:      C.uint32_t(object.Version),
		bucket:       newBucketInfo(&object.Bucket),
		path:         C.CString(object.Path),
		is_prefix:    C.bool(object.IsPrefix),
		metadata:     C.MapRef{universe.Add(object.Metadata)},
		content_type: C.CString(object.ContentType),
		created:      C.int64_t(object.Created.Unix()),
		modified:     C.int64_t(object.Modified.Unix()),
		expires:      C.int64_t(object.Expires.Unix()),
	}
}

// convertEncryptionParameters converts Go EncryptionParameters to C.
func convertEncryptionParameters(goParams *storj.EncryptionParameters) C.EncryptionParameters {
	return C.EncryptionParameters{
		cipher_suite: C.uint8_t(goParams.CipherSuite),
		block_size:   C.int32_t(goParams.BlockSize),
	}
}

// convertRedundancyScheme converts Go RedundancyScheme to C.
func convertRedundancyScheme(scheme *storj.RedundancyScheme) C.RedundancyScheme {
	return C.RedundancyScheme{
		algorithm:       C.uint8_t(scheme.Algorithm),
		share_size:      C.int32_t(scheme.ShareSize),
		required_shares: C.int16_t(scheme.RequiredShares),
		repair_shares:   C.int16_t(scheme.RepairShares),
		optimal_shares:  C.int16_t(scheme.OptimalShares),
		total_shares:    C.int16_t(scheme.TotalShares),
	}
}

// bytes_to_cbytes converts a byte array to a C uint8_t array
func bytes_to_cbytes(bytes []byte) (data *C.uint8_t, _ C.uint64_t) {
	length := len(bytes)
	ptr := uintptr(C.malloc(C.size_t(length)))
	mem := (*[1 << 30]uint8)(unsafe.Pointer(ptr))
	copy((*mem)[:], bytes)

	return data, C.uint64_t(length)
}
