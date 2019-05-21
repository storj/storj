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
	"reflect"
	"unsafe"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/lib/uplink/ext/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// cgo types
	cStringType = reflect.TypeOf(C.CString(""))
	cBoolType   = reflect.TypeOf(C.bool(false))
	cIntType    = reflect.TypeOf(C.int(0))
	cUintType   = reflect.TypeOf(C.uint(0))
	// NB: C.uchar is uint8
	cUcharType = reflect.TypeOf(C.uchar('0'))
	// NB: C.long is int64
	cLongType  = reflect.TypeOf(C.long(0))
	cUlongType = reflect.TypeOf(C.ulong(0))

	// our types
	memorySizeType          = reflect.TypeOf(memory.Size(0))
	cipherSuiteType         = reflect.TypeOf(storj.CipherSuite(0))
	redundancyAlgorithmType = reflect.TypeOf(storj.RedundancyAlgorithm(0))
	keyPtrType              = reflect.TypeOf(new(C.Key))
	goValueType             = reflect.TypeOf(C.struct_GoValue{})
	cGoUintptrType          = reflect.TypeOf(C.GoUintptr(0))

	ErrConvert  = errs.Class("struct conversion error")
	ErrSnapshot = errs.Class("unable to snapshot value")

	structRefMap = newMapping()
)

//export GetIDVersion
func GetIDVersion(number C.uint, cErr **C.char) (cIDVersion C.gvIDVersion) {
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cIDVersion
	}

	return C.gvIDVersion{
		Ptr:  C.IDVersionRef(structRefMap.Add(goIDVersion)),
		Type: C.IDVersionType,
	}
}

// Create pointer to a go struct for C to interact with
func cPointerFromGoStruct(s interface{}) C.GoUintptr {
	return C.GoUintptr(reflect.ValueOf(s).Pointer())
}

func goPointerFromCGoUintptr(p C.GoUintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p))
}

type GoValue struct {
	ptr      token
	_type    uint32
	snapshot []byte
	size     uintptr
}

// Snapshot will look up a struct in the structRefMap, convert it to a protobuf value, and serialize that data into the govalue
func (val GoValue) Snapshot() (data []byte, _ error) {
	protoMsg, err := ToProtoStruct(val.ptr, val._type)
	if err != nil {
		return data, err
	}

	return proto.Marshal(protoMsg)
}

func ToProtoStruct(structRef token, valtype C.enum_ValueType) (protoStruct proto.Message, err error) {
	switch valtype {
	case C.IDVersionType:
		uplinkStruct := structRefMap.Get(structRef).(storj.IDVersion)
		return &pb.IDVersion{
			Number: uint32(uplinkStruct.Number),
		}, nil
	case C.UplinkConfigType:
		uplinkStruct := structRefMap.Get(structRef).(uplink.Config)

		return &pb.UplinkConfig {
			Tls: &pb.TLSConfig{
				SkipPeerCaWhitelist: uplinkStruct.Volatile.TLS.SkipPeerCAWhitelist,
				PeerCaWhitelistPath: uplinkStruct.Volatile.TLS.PeerCAWhitelistPath,
			},
			IdentityVersion: &pb.IDVersion {
				Number: uint32(uplinkStruct.Volatile.IdentityVersion.Number),
			},
			MaxInlineSize: int64(uplinkStruct.Volatile.MaxInlineSize),
			MaxMemory:     int64(uplinkStruct.Volatile.MaxMemory),
		}, nil
	default:
		// TODO: rename `ErrConvert` to `ErrValue` or something and change message accordingly
		return nil, fmt.Errorf("type %s", valtype)
	}
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

// SendToGo takes a GoValue containing a serialized protobuf snapshot and deserializes
// it into a struct in go memory. Then that struct is put in the struct reference map
// and the GoValue ptr field is updated accordingly.
//export SendToGo
func SendToGo(cVal *C.struct_GoValue, cErr **C.char) {
	var msg proto.Message

	switch cVal.Type {
	case C.UplinkConfigType:
		msg = &pb.UplinkConfig{}
	default:
		*cErr = C.CString(errs.New("unsupported type").Error())
		return
	}

	snapshot := make([]byte, int(cVal.Size))
	// TODO: Clean this
	cursor := uintptr(unsafe.Pointer(cVal.Snapshot))
	for i := 0; i < int(cVal.Size); i++ {
		address := cursor + uintptr(i)
		snapshot[i] = *(*byte)(unsafe.Pointer(address))
	}

	if err := proto.Unmarshal(snapshot, msg); err != nil {
		*cErr = C.CString(err.Error())
		return
	}

	fmt.Println(msg)

	cVal.Ptr = C.GoUintptr(structRefMap.Add(msg))
}

func CMalloc(size uintptr) uintptr {
	CMem := C.malloc(C.ulong(size))
	return uintptr(CMem)
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

// GoToCGoValue will return a C equivalent of a go value struct with a populated snapshot
func (gv GoValue) GoToCGoValue() (cVal C.struct_GoValue, err error) {
	return C.struct_GoValue{
		Ptr:      C.GoUintptr(gv.ptr),
		Type:     C.enum_ValueType(gv._type),
		Snapshot: (*C.uchar)(unsafe.Pointer(&gv.snapshot)),
		Size:     C.GoUintptr(gv.size),
	}, nil
}
