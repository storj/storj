// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

//go:generate go build -o uplink-cgo-common.so -buildmode=c-shared apikey.go
//go:generate swig -go -intgosize 64 -module main -o uplink uplink-cgo.h

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include "uplink.h"
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

func main() {

}
