// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"

	libuplink "storj.io/storj/lib/uplink"
)

//export new_scope
// new_scope creates new Scope
func new_scope(satelliteAddress *C.char, apikeyRef C.APIKeyRef, encAccessRef C.EncryptionAccessRef, cerr **C.char) C.ScopeRef {
	apikey, ok := universe.Get(apikeyRef._handle).(libuplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
		return C.ScopeRef{}
	}

	encAccess, ok := universe.Get(encAccessRef._handle).(*libuplink.EncryptionAccess)
	if !ok {
		*cerr = C.CString("invalid encryption access")
		return C.ScopeRef{}
	}

	scope := &libuplink.Scope{
		SatelliteAddr:    C.GoString(satelliteAddress),
		APIKey:           apikey,
		EncryptionAccess: encAccess,
	}
	return C.ScopeRef{_handle: universe.Add(scope)}
}

//export get_scope_satellite_address
// get_scope_satellite_address gets Scope satellite address
func get_scope_satellite_address(scopeRef C.ScopeRef, cerr **C.char) *C.char {
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return nil
	}

	return C.CString(scope.SatelliteAddr)
}

//export get_scope_api_key
// get_scope_api_key gets Scope APIKey
func get_scope_api_key(scopeRef C.ScopeRef, cerr **C.char) C.APIKeyRef {
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return C.APIKeyRef{}
	}

	return C.APIKeyRef{_handle: universe.Add(scope.APIKey)}
}

//export get_scope_enc_access
// get_scope_enc_access gets Scope encryption access
func get_scope_enc_access(scopeRef C.ScopeRef, cerr **C.char) C.EncryptionAccessRef {
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return C.EncryptionAccessRef{}
	}

	return C.EncryptionAccessRef{_handle: universe.Add(scope.EncryptionAccess)}
}

//export parse_scope
// parse_scope parses an Scope
func parse_scope(val *C.char, cerr **C.char) C.ScopeRef {
	scope, err := libuplink.ParseScope(C.GoString(val))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.ScopeRef{}
	}

	return C.ScopeRef{_handle: universe.Add(scope)}
}

//export serialize_scope
// serialize_scope serializes the Scope to a string
func serialize_scope(scopeRef C.ScopeRef, cerr **C.char) *C.char {
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return nil
	}

	serializedScope, err := scope.Serialize()
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return nil
	}

	return C.CString(serializedScope)
}

//export free_scope
// free_scope frees an scope
func free_scope(scopeRef C.ScopeRef) {
	universe.Del(scopeRef._handle)
}
