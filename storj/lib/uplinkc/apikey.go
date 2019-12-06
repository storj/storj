// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"

	libuplink "storj.io/storj/lib/uplink"
)

//export parse_api_key
// parse_api_key parses an API Key
func parse_api_key(val *C.char, cerr **C.char) C.APIKeyRef {
	apikey, err := libuplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.APIKeyRef{}
	}

	return C.APIKeyRef{universe.Add(apikey)}
}

//export serialize_api_key
// serialize_api_key serializes the API Key to a string
func serialize_api_key(apikeyHandle C.APIKeyRef, cerr **C.char) *C.char {
	apikey, ok := universe.Get(apikeyHandle._handle).(libuplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
		return nil
	}

	return C.CString(apikey.Serialize())
}

//export free_api_key
// free_api_key frees an api key
func free_api_key(apikeyHandle C.APIKeyRef) {
	universe.Del(apikeyHandle._handle)
}
