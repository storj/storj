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

//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val *C.char, cErr **C.char) (cApiKey C.gvAPIKey) {
	goApiKeyStruct, err := uplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cApiKey
	}

	return C.gvAPIKey{
		Ptr:  C.IDVersionRef(structRefMap.Add(goApiKeyStruct)),
		Type: C.APIKeyType,
	}
}

//export Serialize
// Serialize serializes the API Key to a string
func Serialize(CApiKey C.APIKeyRef) *C.char {
	goApiKeyStruct, ok := structRefMap.Get(token(CApiKey)).(uplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(goApiKeyStruct.Serialize())
}
