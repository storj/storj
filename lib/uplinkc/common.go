// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "C"
import (
	"sync"
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

//GoCMalloc allocates C memory
func GoCMalloc(size uintptr) uintptr {
	CMem := CMalloc(CSize(size))
	return uintptr(CMem)
}

//export GetIDVersion
func GetIDVersion(number CUint, cErr *CCharPtr) (cIDVersion CIDVersion) {
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = CCString(err.Error())
		return cIDVersion
	}

	return CIDVersion{
		number: CUint16(goIDVersion.Number),
	}
}

type GoMap struct {
	mtx sync.Mutex
	m map[string]string
}

//export NewMapRef
func NewMapRef() CMapRef {
	return CMapRef(universe.Add(&GoMap{}))
}
//export MapRefSet
func MapRefSet(metaDataRef CMapRef, key CCharPtr, value CCharPtr, cErr *CCharPtr) {
	metaData, ok := universe.Get(Token(metaDataRef)).(*GoMap)
	if !ok {
		*cErr = CCString("invalid map")
		return
	}

	metaData.mtx.Lock()
	metaData.m[CGoString(key)] = CGoString(value)
	metaData.mtx.Unlock()
}


//export MapRefGet
func MapRefGet(metaDataRef CMapRef, key CCharPtr, cErr *CCharPtr) (cValue CCharPtr) {
	metaData, ok := universe.Get(Token(metaDataRef)).(*GoMap)
	if !ok {
		*cErr = CCString("invalid map")
		return cValue
	}

	metaData.mtx.Lock()
	value := metaData.m[CGoString(key)]
	metaData.mtx.Unlock()

	return CCString(value)
}

// bytesToCbytes creates a CBytes_t struct from a go bytes array
func bytesToCbytes(bytes []byte, lenOfBytes int, cData *CBytes) {
	ptr := GoCMalloc(uintptr(lenOfBytes))
	mem := unsafe.Pointer(ptr)
	for i := 0; i < lenOfBytes; i++ {
		nextAddress := uintptr(int(ptr) + i)
		*(*uint8)(unsafe.Pointer(nextAddress)) = bytes[i]
	}

	cData.length = CInt32(lenOfBytes)
	cData.bytes = (*CUint8)(mem)
}

func NewCBucket(bucket *storj.Bucket) CBucket {
	encParamsPtr := NewCEncryptionParamsPtr(&bucket.EncryptionParameters)
	redundancySchemePtr := NewCRedundancySchemePtr(&bucket.RedundancyScheme)

	return CBucket{
		encryption_parameters: encParamsPtr,
		redundancy_scheme:     redundancySchemePtr,
		name:                  CCString(bucket.Name),
		// TODO: use `UnixNano()`?
		created:      CInt64(bucket.Created.Unix()),
		path_cipher:  CUint8(bucket.PathCipher),
		segment_size: CInt64(bucket.SegmentsSize),
	}
}

func NewCBucketConfig(bucketCfg *uplink.BucketConfig) CBucketConfig {
	return CBucketConfig{
		encryption_parameters: NewCEncryptionParamsPtr(&bucketCfg.EncryptionParameters),
		redundancy_scheme:     NewCRedundancySchemePtr(&bucketCfg.Volatile.RedundancyScheme),
		path_cipher:           CUint8(bucketCfg.PathCipher),
	}
}

// NB: caller is responsible for freeing memory at `ptr`
func NewCEncryptionParamsPtr(goParams *storj.EncryptionParameters) CEncryptionParameters {
	return CEncryptionParameters{
		cipher_suite: CUint8(goParams.CipherSuite),
		block_size:   CInt32(goParams.BlockSize),
	}
}

// NB: caller is responsible for freeing memory at `ptr`
func NewCRedundancySchemePtr(goScheme *storj.RedundancyScheme) CRedundancyScheme {
	return CRedundancyScheme{
		algorithm:       CUint8(goScheme.Algorithm),
		share_size:      CInt32(goScheme.ShareSize),
		required_shares: CInt16(goScheme.RequiredShares),
		repair_shares:   CInt16(goScheme.RepairShares),
		optimal_shares:  CInt16(goScheme.OptimalShares),
		total_shares:    CInt16(goScheme.TotalShares),
	}
}

//export FreeReference
func FreeReference(reference Token) {
	universe.Del(reference)
}