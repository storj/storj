// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"storj.io/storj/lib/uplink"
)

// typedef __SIZE_TYPE__ APIKeyRef_t;
import "C"

//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val *C.char, cErr **C.char) (cAPIKey cAPIKeyRef) {
	apikey, err := uplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cAPIKey
	}

	return cAPIKeyRef(universe.Add(apikey))
}

//export Serialize
// Serialize serializes the API Key to a string
func Serialize(cAPIKey cAPIKeyRef) *C.char {
	goApiKey, ok := universe.Get(Ref(cAPIKey)).(uplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(goApiKey.Serialize())
}