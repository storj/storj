// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"
	"time"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
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

// export restrict_api_key
// restrict_api_key generates a new APIKey with the provided Caveat attached.
func restrict_api_key(apikeyRef C.APIKeyRef, cCaveat C.Caveat, cerr **C.char) C.APIKeyRef {
	apikey, ok := universe.Get(apikeyRef._handle).(libuplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
	}

	allowedPaths, ok := universe.Get(cCaveat.allowed_paths._handle).([]*macaroon.Caveat_Path)
	if !ok {
		*cerr = C.CString("invalid allowed caveat paths")
	}

	nonce, ok := universe.Get(cCaveat.nonce._handle).([]byte)
	if !ok {
		*cerr = C.CString("invalid caveat nonce")
	}

	notAfter := time.Unix(int64(cCaveat.not_after), 0)
	notBefore := time.Unix(int64(cCaveat.not_before), 0)

	caveat := macaroon.Caveat{
		DisallowReads:   bool(cCaveat.disallow_reads),
		DisallowWrites:  bool(cCaveat.disallow_writes),
		DisallowLists:   bool(cCaveat.disallow_lists),
		DisallowDeletes: bool(cCaveat.disallow_deletes),
		AllowedPaths:    allowedPaths,
		NotAfter:        &notAfter,
		NotBefore:       &notBefore,
		Nonce:           nonce,
	}
	restrictedAPIKey, err := apikey.Restrict(caveat)
	if err != nil {
		*cerr = C.CString(err.Error())
	}
	return C.APIKeyRef{universe.Add(restrictedAPIKey)}
}

//export free_api_key
// free_api_key frees an api key
func free_api_key(apikeyHandle C.APIKeyRef) {
	universe.Del(apikeyHandle._handle)
}
