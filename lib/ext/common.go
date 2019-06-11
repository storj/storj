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
	"sync"
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var structRefMap = newMapping()

//CMalloc allocates C memory
func CMalloc(size uintptr) uintptr {
	CMem := C.malloc(C.size_t(size))
	return uintptr(CMem)
}

// GetIDVersion looks up the given version number in the map of registered
// versions, returning an error if none is found.
//export GetIDVersion
func GetIDVersion(number C.uint, cErr **C.char) (cIDVersion C.IDVersion_t) {
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cIDVersion
	}

	return C.IDVersion_t{
		number: C.uint16_t(goIDVersion.Number),
	}
}

type MapRef struct {
	mtx sync.Mutex
	m map[string]string
}

// NewMapRef returns a new ref/handle to a go map[string]string.
//export NewMapRef
func NewMapRef() C.MapRef_t {
	mapref := &MapRef{}
	mapref.m = make(map[string]string)
	return C.MapRef_t(structRefMap.Add(mapref))
}

// MapRefSet sets the passed key to the passed value in the go map that the passed ref refers to.
//export MapRefSet
func MapRefSet(metaDataRef C.MapRef_t, key *C.char, value *C.char, cErr **C.char) {
	metaData, ok := structRefMap.Get(token(metaDataRef)).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
		return
	}

	metaData.mtx.Lock()
	metaData.m[C.GoString(key)] = C.GoString(value)
	metaData.mtx.Unlock()
}

// MapRefGet gets the value of the passed key in the go map that the passed ref refers to.
//export MapRefGet
func MapRefGet(metaDataRef C.MapRef_t, key *C.char, cErr **C.char) (cValue *C.char) {
	metaData, ok := structRefMap.Get(token(metaDataRef)).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
		return cValue
	}

	metaData.mtx.Lock()
	value := metaData.m[C.GoString(key)]
	metaData.mtx.Unlock()

	return C.CString(value)
}

// bytesToCbytes creates a C.Bytes_t struct from a go bytes array
func bytesToCbytes(bytes []byte, lenOfBytes int, cData *C.Bytes_t) {
	ptr := CMalloc(uintptr(lenOfBytes))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < lenOfBytes; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = bytes[i]
	}

	cData.length = C.int32_t(lenOfBytes)
	cData.bytes = (*C.uint8_t)(mem)
}

// newBucketInfo returns a C bucket struct converted from a go bucket struct.
func newBucketInfo(bucket *storj.Bucket) C.BucketInfo {
	return C.BucketInfo{
		name:         C.CString(bucket.Name),
		created:      C.int64_t(bucket.Created.Unix()),
		path_cipher:  C.uint8_t(bucket.PathCipher),
		segment_size: C.int64_t(bucket.SegmentsSize),

		encryption_parameters: NewCEncryptionParams(&bucket.EncryptionParameters),
		redundancy_scheme:     NewCRedundancyScheme(&bucket.RedundancyScheme),
	}
}

// FreeBucketInfo frees bucket info.
//export FreeBucketInfo
func FreeBucketInfo(bucketInfo *C.BucketInfo) {
	C.free(bucketInfo.name)
	bucketInfo.name = nil
}

// NewCBucketConfig returns a C bucket config struct converted from a go bucket config struct.
func NewCBucketConfig(bucketCfg *uplink.BucketConfig) C.BucketConfig_t {
	return C.BucketConfig_t{
		encryption_parameters: NewCEncryptionParams(&bucketCfg.EncryptionParameters),
		redundancy_scheme:     NewCRedundancyScheme(&bucketCfg.Volatile.RedundancyScheme),
		path_cipher:           CUint8(bucketCfg.PathCipher),
	}
}

// NewCEncryptionParams returns a C encryption parameters struct converted from a
// go encryption parameters struct.
func NewCEncryptionParams(goParams *storj.EncryptionParameters) C.EncryptionParameters_t {
	return C.EncryptionParameters_t{
		cipher_suite: C.uint8_t(goParams.CipherSuite),
		block_size:   C.int32_t(goParams.BlockSize),
	}
}

// NewCRedundancyScheme returns a C redundancy scheme struct converted from a
// go redundancy scheme struct.
func NewCRedundancyScheme(goScheme *storj.RedundancyScheme) C.RedundancyScheme_t {
	return C.RedundancyScheme_t{
		algorithm:       C.uint8_t(goScheme.Algorithm),
		share_size:      C.int32_t(goScheme.ShareSize),
		required_shares: C.int16_t(goScheme.RequiredShares),
		repair_shares:   C.int16_t(goScheme.RepairShares),
		optimal_shares:  C.int16_t(goScheme.OptimalShares),
		total_shares:    C.int16_t(goScheme.TotalShares),
	}
}

// FreeReference deletes the passed reference from the struct map.
//export FreeReference
func FreeReference(reference token) {
	structRefMap.Del(reference)
}