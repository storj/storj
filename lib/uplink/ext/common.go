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
	"storj.io/storj/pkg/storj"
)

var (
	structRefMap = newMapping()
)

//CMalloc allocates C memory
func CMalloc(size uintptr) uintptr {
	CMem := C.malloc(C.ulong(size))
	return uintptr(CMem)
}

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