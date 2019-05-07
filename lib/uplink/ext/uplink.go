// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "uplink.h"
// #endif
import "C"
import (
	"context"
	"reflect"

	"storj.io/storj/lib/uplink"
)

//export NewUplink
func NewUplink(cConfig C.struct_Config, cErr **C.char) (cUplink C.struct_Uplink) {
	goConfig := new(uplink.Config)
	if err := CToGoStruct(cConfig, goConfig); err != nil {
		*cErr = C.CString(err.Error())
		return cUplink
	}

	goUplink, err := uplink.NewUplink(context.Background(), goConfig)
	if err != nil {
		*cErr = C.CString(err.Error())
		return cUplink
	}

	return C.struct_Uplink{
		GoUplink: C.GoUintptr(reflect.ValueOf(goUplink).Pointer()),
		Config:   cConfig,
	}
	//if err := GoToCStruct(goUplink, &cUplink); err != nil {
	//	*cErr = C.CString(err.Error())
	//	return C.struct_Uplink{}
	//}
	//return cUplink
}


var cRegistry = make(map[uint64]interface{})
var cNext uint64 = 0

func register(value interface{}) uint64 {
	cNext += 1
	cRegistry[cNext] = value
	return cNext
}

func lookup(key uint64) interface{} {
	return cRegistry[key]
}
