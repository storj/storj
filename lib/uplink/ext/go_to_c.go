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
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/lib/uplink/ext/pb"
	"storj.io/storj/pkg/storj"
	"unsafe"
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
	value := CToGoGoValue(*cValue)

	if err := value.Snapshot(); err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	if err := value.GoToCGoValue(cValue); err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	fmt.Println(value)
	fmt.Println(cValue)
}

// Snapshot
// 	look up a struct in the structRefMap
// 	convert it to a protobuf value
// 	serialize that data into the govalue
func (gv *GoValue) Snapshot() (err error) {
	var data []byte
	switch gv._type {
	case C.IDVersionType:
		uplinkStruct := structRefMap.Get(gv.ptr).(storj.IDVersion)
		data, err = proto.Marshal(&pb.IDVersion{
			Number: uint32(uplinkStruct.Number),
		})
		break
	case C.UplinkConfigType:
		uplinkConfigStruct := structRefMap.Get(gv.ptr).(*uplink.Config)

		data, err = proto.Marshal(&pb.UplinkConfig{
			Tls: &pb.TLSConfig{
				SkipPeerCaWhitelist: uplinkConfigStruct.Volatile.TLS.SkipPeerCAWhitelist,
				PeerCaWhitelistPath: uplinkConfigStruct.Volatile.TLS.PeerCAWhitelistPath,
			},
			IdentityVersion: &pb.IDVersion{
				Number: uint32(uplinkConfigStruct.Volatile.IdentityVersion.Number),
			},
			MaxInlineSize: int64(uplinkConfigStruct.Volatile.MaxInlineSize),
			MaxMemory:     int64(uplinkConfigStruct.Volatile.MaxMemory),
		})
		break
	case C.BucketType:
		bucketStruct := structRefMap.Get(gv.ptr).(*storj.Bucket)

		data, err = proto.Marshal(&pb.Bucket{
			Name: bucketStruct.Name,
			RedundancyScheme: &pb.RedundancyScheme{
				Algorithm:      uint32(bucketStruct.RedundancyScheme.Algorithm),
				TotalShares:    int32(bucketStruct.RedundancyScheme.TotalShares),
				ShareSize:      bucketStruct.RedundancyScheme.ShareSize,
				RequiredShares: int32(bucketStruct.RedundancyScheme.RequiredShares),
				RepairShares:   int32(bucketStruct.RedundancyScheme.RepairShares),
				OptimalShares:  int32(bucketStruct.RedundancyScheme.OptimalShares),
			},
			SegmentSize: int64(bucketStruct.SegmentsSize),
			EncryptionParameters: &pb.EncryptionParameters{
				CipherSuite: uint32(bucketStruct.EncryptionParameters.CipherSuite),
				BlockSize:   bucketStruct.EncryptionParameters.BlockSize,
			},
			PathCipher: uint32(bucketStruct.PathCipher), Created: uint64(bucketStruct.Created.Unix()),
		})
		break
	default:
		return ErrSnapshot.New("type", gv._type)
	}

	gv.size = uintptr(len(data))
	gv.snapshot = data

	return nil
}

// GoToCGoValue will return a C equivalent of a go value struct with a populated snapshot
func (gv *GoValue) GoToCGoValue(cVal *C.struct_GoValue) error {
	cVal.Ptr = C.GoUintptr(gv.ptr)
	cVal.Type = C.enum_ValueType(gv._type)
	cVal.Size = C.GoUintptr(gv.size)

	ptr := CMalloc(gv.size)
	mem := (*[]byte)(unsafe.Pointer(ptr))
	// data will be empty if govalue only has defaults
	if gv.size > 0 {
		copy(*mem, gv.snapshot)
	}

	cVal.Snapshot = (*C.uchar)(unsafe.Pointer(mem))

	return nil
}
