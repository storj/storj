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
	"storj.io/storj/lib/uplink"
)

// ParseAPIKey parses an API Key
//export ParseAPIKey
func ParseAPIKey(val *C.char, cErr **C.char) (cAPIKey C.APIKeyRef_t) {
	goAPIKeyStruct, err := uplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cAPIKey
	}

	return C.APIKeyRef_t(structRefMap.Add(goAPIKeyStruct))
}

// Serialize serializes the API Key to a string
//export Serialize
func Serialize(cAPIKey C.APIKeyRef_t) *C.char {
	goAPIKey, ok := structRefMap.Get(token(cAPIKey)).(uplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(goAPIKey.Serialize())
}
