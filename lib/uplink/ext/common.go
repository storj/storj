// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef STORJ_HEADERS
//   #define STORJ_HEADERS
//   #include "c/headers/main.h"
// #endif
import "C"
import (
	"bytes"
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

//export NewBuffer
func NewBuffer() (cBuffer C.BufferRef_t) {
	return C.BufferRef_t(structRefMap.Add(new(bytes.Buffer)))
}

//export WriteBuffer
func WriteBuffer(cBuffer C.BufferRef_t, cData *C.Bytes_t, cErr **C.char) {
	buf, ok := structRefMap.Get(token(cBuffer)).(*bytes.Buffer)
	if !ok {
		*cErr = C.CString("invalid buffer")
		return
	}

	data := C.GoBytes(unsafe.Pointer(cData.bytes), C.int(cData.length))
	if _, err := buf.Write(data); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

//export ReadBuffer
func ReadBuffer(cBuffer C.BufferRef_t, cData *C.Bytes_t, cErr **C.char) {
	buf, ok := structRefMap.Get(token(cBuffer)).(*bytes.Buffer)
	if !ok {
		*cErr = C.CString("invalid buffer")
		return
	}

	bufLen := buf.Len()
	cData.length = C.int32_t(bufLen)

	ptr := CMalloc(uintptr(bufLen))
	mem := unsafe.Pointer(ptr)
	data := buf.Bytes()
	for i := 0; i < bufLen; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = data[i]
	}
	cData.bytes = (*C.uint8_t)(mem)
}

func bytesToCbytes(bytes []byte) (cData *C.Bytes_t) {
	cData = new(C.Bytes_t)
	lenOfBytes := len(bytes)

	ptr := CMalloc(uintptr(lenOfBytes))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < lenOfBytes; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = bytes[i]
	}

	cData.length = C.int32_t(lenOfBytes)
	cData.bytes = (*C.uint8_t)(mem)

	return cData
}

func NewCBucket(bucket *storj.Bucket) C.Bucket_t {
	encParamsPtr := NewCEncryptionParamsPtr(&bucket.EncryptionParameters)
	redundancySchemePtr := NewCRedundancySchemePtr(&bucket.RedundancyScheme)

	return C.Bucket_t{
		encryption_parameters: encParamsPtr,
		redundancy_scheme:     redundancySchemePtr,
		name:                  C.CString(bucket.Name),
		// TODO: use `UnixNano()`?
		created:      C.int64_t(bucket.Created.Unix()),
		path_cipher:  C.uint8_t(bucket.PathCipher),
		segment_size: C.int64_t(bucket.SegmentsSize),
	}
}

func NewCBucketConfig(bucketCfg *uplink.BucketConfig) C.BucketConfig_t {
	return C.BucketConfig_t{
		encryption_parameters: NewCEncryptionParamsPtr(&bucketCfg.EncryptionParameters),
		redundancy_scheme: NewCRedundancySchemePtr(&bucketCfg.Volatile.RedundancyScheme),
		path_cipher:           CUint8(bucketCfg.PathCipher),
	}
}

// NB: caller is responsible for freeing memory at `ptr`
func NewCEncryptionParamsPtr(goParams *storj.EncryptionParameters) *C.EncryptionParameters_t {
	ptr := CMalloc(unsafe.Sizeof(C.EncryptionParameters_t{}))
	cParams := (*C.EncryptionParameters_t)(unsafe.Pointer(ptr))
	*cParams = C.EncryptionParameters_t{
		cipher_suite: C.uint8_t(goParams.CipherSuite),
		block_size:   C.int32_t(goParams.BlockSize),
	}
	return cParams
}

// NB: caller is responsible for freeing memory at `ptr`
func NewCRedundancySchemePtr(goScheme *storj.RedundancyScheme) *C.RedundancyScheme_t {
	ptr := CMalloc(unsafe.Sizeof(C.RedundancyScheme_t{}))
	cScheme := (*C.RedundancyScheme_t)(unsafe.Pointer(ptr))
	*cScheme = C.RedundancyScheme_t{
		algorithm:       C.uint8_t(goScheme.Algorithm),
		share_size:      C.int32_t(goScheme.ShareSize),
		required_shares: C.int16_t(goScheme.RequiredShares),
		repair_shares:   C.int16_t(goScheme.RepairShares),
		optimal_shares:  C.int16_t(goScheme.OptimalShares),
		total_shares:    C.int16_t(goScheme.TotalShares),
	}
	return cScheme
}

//export FreeReference
func FreeReference(reference token) {
	structRefMap.Del(reference)
}