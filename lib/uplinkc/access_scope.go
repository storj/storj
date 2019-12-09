// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"
	"unsafe"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
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
func serialize_scope(scopeHandle C.ScopeRef, cerr **C.char) *C.char {
	scope, ok := universe.Get(scopeHandle._handle).(*libuplink.Scope)
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

//export restrict_scope
// restrict_scope restricts a given scope with the provided caveat and encryptionRestrictions
func restrict_scope(scopeRef C.ScopeRef, caveat C.Caveat, restrictions **C.EncryptionRestriction, cerr **C.char) C.ScopeRef {
	//Get scope
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return C.ScopeRef{}
	}

	//Get caveat from C
	if(!caveat){
		*cerr = C.CString("invalid caveat")
		return C.ScopeRef{}
	}
	caveatGo := macaroon.Caveat{
		DisallowReads:   caveat.disallow_reads == C.bool(true),
		DisallowWrites:  caveat.disallow_writes == C.bool(true),
		DisallowLists:   caveat.disallow_lists == C.bool(true),
		DisallowDeletes: caveat.disallow_deletes == C.bool(true),
	}

	//Restrict apiKey using caveat
	apiKeyRestricted, err := scope.APIKey.Restrict(caveatGo)
	if err != nil {
		*cerr = C.CString("could not restrict apiKey")
		return C.ScopeRef{}
	}

	//Convert EncryptionRestrictions to Go
	restrictionsArray := (*[1 << 30 / unsafe.Sizeof(C.EncryptionRestriction{})]C.EncryptionRestriction)(unsafe.Pointer(restrictions))
	restrictionsGo := make([]libuplink.EncryptionRestriction, 0)
	for i := 0; i < len(restrictionsArray); i++ {
		restrictionsGo = append(restrictionsGo, libuplink.EncryptionRestriction{
			Bucket:     C.GoString(restrictionsArray[i].bucket),
			PathPrefix: C.GoString(restrictionsArray[i].path_prefix),
		})
	}

	//Create new EncryptionAccess with restrictions
	apiKeyRestricted, encAccessRestricted, err := scope.EncryptionAccess.Restrict(apiKeyRestricted, restrictionsGo...)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.ScopeRef{}
	}

	scopeRestricted := &libuplink.Scope{
		SatelliteAddr:    scope.SatelliteAddr,
		APIKey:           apiKeyRestricted,
		EncryptionAccess: encAccessRestricted,
	}
	return C.ScopeRef{_handle: universe.Add(scopeRestricted)}
}

//export free_scope
// free_scope frees an scope
func free_scope(scopeHandle C.ScopeRef) {
	universe.Del(scopeHandle._handle)
}
