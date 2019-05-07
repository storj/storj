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
}
