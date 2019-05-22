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
	"unsafe"
	
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/lib/uplink/ext/pb"
	"storj.io/storj/pkg/storj"
)

var (
	ErrSnapshot = errs.Class("unable to snapshot value")
)

type GoValue struct {
	ptr      token
	_type    uint32
	snapshot []byte
	size     uintptr
}

// GetSnapshot will take a C GoValue struct that was created in go and populate the snapshot
//export CGetSnapshot
func CGetSnapshot(cValue *C.struct_GoValue, cErr **C.char) {
	govalue := CToGoGoValue(*cValue)

	if err := govalue.GetSnapshot(); err != nil {
		*cErr = C.CString(err.Error())
		return
	}
}

// Snapshot
// 	look up a struct in the structRefMap
// 	convert it to a protobuf value
// 	serialize that data into the govalue
func (gv GoValue) Snapshot() (data []byte, _ error) {
	switch gv._type {
	case C.IDVersionType:
		uplinkStruct := structRefMap.Get(gv.ptr).(storj.IDVersion)
		return proto.Marshal(&pb.IDVersion{
			Number: uint32(uplinkStruct.Number),
		})
	case C.UplinkConfigType:
		uplinkConfigStruct := structRefMap.Get(gv.ptr).(uplink.Config)

		return proto.Marshal(&pb.UplinkConfig {
			Tls: &pb.TLSConfig{
				SkipPeerCaWhitelist: uplinkConfigStruct.Volatile.TLS.SkipPeerCAWhitelist,
				PeerCaWhitelistPath: uplinkConfigStruct.Volatile.TLS.PeerCAWhitelistPath,
			},
			IdentityVersion: &pb.IDVersion {
				Number: uint32(uplinkConfigStruct.Volatile.IdentityVersion.Number),
			},
			MaxInlineSize: int64(uplinkConfigStruct.Volatile.MaxInlineSize),
			MaxMemory:     int64(uplinkConfigStruct.Volatile.MaxMemory),
		})
	case C.BucketType:
		bucketStruct := structRefMap.Get(gv.ptr).(*uplink.Bucket)

		return proto.Marshal(&pb.Bucket {
			BucketConfig: &pb.BucketConfig{
				RedundancyScheme: &pb.RedundancyScheme{
					Algorithm: uint32(bucketStruct.BucketConfig.Volatile.RedundancyScheme.Algorithm),
					TotalShares: int32(bucketStruct.BucketConfig.Volatile.RedundancyScheme.TotalShares),
					ShareSize: bucketStruct.BucketConfig.Volatile.RedundancyScheme.ShareSize,
					RequiredShares: int32(bucketStruct.BucketConfig.Volatile.RedundancyScheme.RequiredShares),
					RepairShares: int32(bucketStruct.BucketConfig.Volatile.RedundancyScheme.RepairShares),
					OptimalShares: int32(bucketStruct.BucketConfig.Volatile.RedundancyScheme.OptimalShares),
				},
				SegmentSize: int64(bucketStruct.BucketConfig.Volatile.SegmentsSize),
				EncryptionParameters: &pb.EncryptionParameters{
					CipherSuite: uint32(bucketStruct.BucketConfig.EncryptionParameters.CipherSuite),
					BlockSize: bucketStruct.BucketConfig.EncryptionParameters.BlockSize,
				},
				PathCipher: uint32(bucketStruct.BucketConfig.PathCipher),
			},
			Name: bucketStruct.Name,
			Created: uint64(bucketStruct.Created.Unix()),
		})
	default:
		return nil, ErrSnapshot.New("type", gv._type)
	}
}

func (gv GoValue) GetSnapshot() error {
	data, err := gv.Snapshot()
	if err != nil {
		return err
	}

	size := uintptr(len(data))
	ptr := CMalloc(size)
	mem := (*[]byte)(unsafe.Pointer(ptr))
	// data will be empty if govalue only has defaults
	if size > 0 {
		copy(*mem, data)
	}
	gv.snapshot = *mem

	return nil
}

// GoToCGoValue will return a C equivalent of a go value struct with a populated snapshot
func (gv GoValue) GoToCGoValue() (cVal C.struct_GoValue, err error) {
	return C.struct_GoValue{
		Ptr:      C.GoUintptr(gv.ptr),
		Type:     C.enum_ValueType(gv._type),
		Snapshot: (*C.uchar)(unsafe.Pointer(&gv.snapshot)),
		Size:     C.GoUintptr(gv.size),
	}, nil
}
