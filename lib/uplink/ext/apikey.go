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

//export TestMe
func TestMe() {

}
