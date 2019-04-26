// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// go build -o awesome.so -buildmode=c-shared common.go apikey.go

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include "structs.h"
import (
	"C"
	// "storj.io/storj/lib/uplink"
)


//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val string) (key C.struct_APIKey) {
	cval := C.CString(val)
	return C.struct_APIKey{
		key: cval,
	}

}

//export Serialize
// Serialize serializes the API Key to a string
func Serialize(key C.struct_APIKey) *C.char {
	return key.key
}
