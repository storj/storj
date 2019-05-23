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

	"storj.io/storj/internal/memory"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/lib/uplink/ext/pb"
	"storj.io/storj/pkg/storj"
)

// SendToGo takes a GoValue containing a serialized protobuf snapshot and deserializes
// it into a struct in go memory. Then that struct is put in the struct reference map
// and the GoValue ptr field is updated accordingly.
//export SendToGo
func SendToGo(cVal *C.struct_GoValue, cErr **C.char) {
	var msg proto.Message

	switch cVal.Type {
	case C.UplinkConfigType:
		msg = &pb.UplinkConfig{}

		if err := unmarshalCSnapshot(cVal, msg); err != nil {
			*cErr = C.CString(err.Error())
			return
		}

		pbConfig := msg.(*pb.UplinkConfig)
		idVersion, err := storj.GetIDVersion(storj.IDVersionNumber(pbConfig.IdentityVersion.Number))
		if err != nil {
			*cErr = C.CString(err.Error())
			return
		}

		cVal.Ptr = C.ulong(structRefMap.Add(&uplink.Config{
			Volatile: uplink.Volatile{
				TLS: struct{
					SkipPeerCAWhitelist bool
					PeerCAWhitelistPath string
				}{
					SkipPeerCAWhitelist: pbConfig.Tls.SkipPeerCaWhitelist,
					PeerCAWhitelistPath: pbConfig.Tls.PeerCaWhitelistPath,
				},
				IdentityVersion: idVersion,
				MaxInlineSize: memory.Size(pbConfig.MaxInlineSize),
				MaxMemory: memory.Size(pbConfig.MaxMemory),
			},
		}))
	case C.ProjectOptionsType:
		msg = &pb.ProjectOptions{}

		if err := unmarshalCSnapshot(cVal, msg); err != nil {
			*cErr = C.CString(err.Error())
			return
		}

		pbOpts := msg.(*pb.ProjectOptions)
		key := new(storj.Key)
		copy((*key)[:], pbOpts.EncryptionKey)

		cVal.Ptr = C.ulong(structRefMap.Add(&uplink.ProjectOptions{
			Volatile: struct {
				EncryptionKey *storj.Key
			} {
				EncryptionKey: key,
			},
		}))
	case C.BucketConfigType:
		msg = &pb.BucketConfig{}

		if err := unmarshalCSnapshot(cVal, msg); err != nil {
			*cErr = C.CString(err.Error())
			return
		}

		pbConfig := msg.(*pb.BucketConfig)

		cVal.Ptr = C.ulong(structRefMap.Add(&uplink.BucketConfig{
			PathCipher: storj.CipherSuite(pbConfig.PathCipher),

			EncryptionParameters: storj.EncryptionParameters{
				BlockSize: pbConfig.EncryptionParameters.BlockSize,
				CipherSuite: storj.CipherSuite(pbConfig.EncryptionParameters.CipherSuite),
			},

			Volatile: struct {
				RedundancyScheme storj.RedundancyScheme
				SegmentsSize memory.Size
			} {
				RedundancyScheme: storj.RedundancyScheme{
					Algorithm: storj.RedundancyAlgorithm(pbConfig.RedundancyScheme.Algorithm),
					OptimalShares: int16(pbConfig.RedundancyScheme.OptimalShares),
					RepairShares: int16(pbConfig.RedundancyScheme.RepairShares),
					RequiredShares: int16(pbConfig.RedundancyScheme.RequiredShares),
					ShareSize: pbConfig.RedundancyScheme.ShareSize,
					TotalShares: int16(pbConfig.RedundancyScheme.TotalShares),
				},
				SegmentsSize: memory.Size(pbConfig.SegmentSize),
			},
		}))
	default:
		*cErr = C.CString(errs.New("unsupported protobuf type").Error())
		return
	}
}

// CToGoGoValue will create a Golang GoValue struct from a C GoValue Struct
func CToGoGoValue(cVal C.struct_GoValue) GoValue {
	snapshot := &[]byte{}
	if cVal.Size > 0 {
		snapshot = (*[]byte)(unsafe.Pointer(cVal.Snapshot))
	}

	return GoValue{
		ptr:   token(cVal.Ptr),
		_type: uint32(cVal.Type),
		snapshot: *snapshot,
		size: uintptr(cVal.Size),
	}
}

func unmarshalCSnapshot(cVal *C.struct_GoValue, msg proto.Message) error {
       snapshot := make([]byte, int(cVal.Size))
       // TODO: Clean this
       cursor := uintptr(unsafe.Pointer(cVal.Snapshot))
       for i := 0; i < int(cVal.Size); i++ {
               address := cursor + uintptr(i)
               snapshot[i] = *(*byte)(unsafe.Pointer(address))
       }

       if err := proto.Unmarshal(snapshot, msg); err != nil {
               return err
       }
       return nil
}