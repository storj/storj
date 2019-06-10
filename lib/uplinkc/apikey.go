// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

/*
#cgo CFLAGS: -g -Wall
*/
import "C"
import (
	"storj.io/storj/lib/uplink"
)

//export ParseAPIKey
// ParseAPIKey parses an API Key
func ParseAPIKey(val CCharPtr, cErr *CCharPtr) (cApiKey CAPIKeyRef) {
	goApiKeyStruct, err := uplink.ParseAPIKey(CGoString(val))
	if err != nil {
		*cErr = CCString(err.Error())
		return cApiKey
	}

	return CAPIKeyRef(universe.Add(goApiKeyStruct))
}

//export Serialize
// Serialize serializes the API Key to a string
func Serialize(cApiKey CAPIKeyRef) CCharPtr {
	goApiKey, ok := universe.Get(Token(cApiKey)).(uplink.APIKey)
	if !ok {
		return CCString("")
	}

	return CCString(goApiKey.Serialize())
}