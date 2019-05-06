// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "uplink.h"
// #endif
import "C"
import (
	"storj.io/storj/lib/uplink/ext/lib"
	"storj.io/storj/pkg/storj"
)

//var (
//	//export GetIDVersion
//	GetIDVersion = storj.GetIDVersion
//)

//export GetIDVersion
func GetIDVersion(number C.uint, cErr **C.char) C.struct_IDVersion {
	cIDVersion := C.struct_IDVersion{}
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = C.CString(err.Error())
		return C.struct_IDVersion{}
	}

	if err := lib.GoToCStruct(goIDVersion, cIDVersion); err != nil {
		*cErr = C.CString(err.Error())
		return cIDVersion
	}
	return cIDVersion
}
