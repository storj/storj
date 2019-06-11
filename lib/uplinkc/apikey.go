// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	libuplink "storj.io/storj/lib/uplink"
)

//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val *C.char, cerr **C.char) (apikeyHandle C.APIKey) {
	apikey, err := libuplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cerr = C.CString(err.Error())
		return apikeyHandle
	}

	return C.APIKey{universe.Add(apikey)}
}

//export SerializeAPIKey
// SerializeAPIKey serializes the API Key to a string
func SerializeAPIKey(apikeyHandle C.APIKey) *C.char {
	apikey, ok := universe.Get(apikeyHandle._handle).(libuplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(apikey.Serialize())
}

//export FreeAPIKey
// FreeAPIKey frees an api key
func FreeAPIKey(apikeyHandle C.APIKey) {
	universe.Del(apikeyHandle._handle)
}