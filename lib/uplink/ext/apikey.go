// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "headers/main.h"
// #endif
import "C"
import (
	"storj.io/storj/lib/uplink"
)

var ( 
	ApiKeyMap = newMapping()
)

//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val *C.char, cErr **C.char) (cApiKey C.APIKey) {
	goApiKeyStruct, err := uplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cApiKey
	}

	return C.APIKey(ApiKeyMap.Add(goApiKeyStruct))

}

//export Serialize
// Serialize serializes the API Key to a string
func Serialize(CApiKey C.APIKey) *C.char {
	goApiKeyStruct, ok := ApiKeyMap.Get(token(CApiKey)).(uplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(goApiKeyStruct.Serialize())
}
