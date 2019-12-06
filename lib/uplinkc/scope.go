// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/private/fpath"
)

// scope implements nesting context for foreign api.
type scope struct {
	ctx    context.Context
	cancel func()
}

// rootScope creates a scope with the specified temp directory.
func rootScope(tempDir string) scope {
	ctx := context.Background()
	if tempDir == "inmemory" {
		ctx = fpath.WithTempData(ctx, "", true)
	} else {
		ctx = fpath.WithTempData(ctx, tempDir, false)
	}
	ctx, cancel := context.WithCancel(ctx)
	return scope{ctx, cancel}
}

// child creates an inherited scope.
func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}

//export restrict_scope
// restricts a given scope with the provided caveat and encryptionRestrictions
func restrict_scope(scopeRef C.ScopeRef, caveatRef C.CaveatRef, restrictions **C.EncryptionRestriction, cerr **C.char) C.ScopeRef {
	//Get apiKey from scope
	scope, ok := universe.Get(scopeRef._handle).(*libuplink.Scope)
	if !ok {
		*cerr = C.CString("invalid scope")
		return C.ScopeRef{}
	}

	//Get caveat from ref
	caveat, ok := universe.Get(caveatRef._handle).(macaroon.Caveat)
	if !ok {
		*cerr = C.CString("invalid caveat")
		return C.ScopeRef{}
	}

	//Restrict apiKey using caveat
	//problem: caveat needs to know about encryptionsRestrictions before vv
	apiKeyRestricted, err := scope.APIKey.Restrict(caveat)
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
	//problem: it uses it's own caveat internally
	_, encAccessRestricted, err := scope.EncryptionAccess.Restrict(apiKeyRestricted, restrictionsGo...)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.ScopeRef{}
	}

	//Construct new scope with the new apikey and restricted encAccess.
	scopeRestricted := &libuplink.Scope{
		SatelliteAddr:    scope.SatelliteAddr,
		APIKey:           apiKeyRestricted,
		EncryptionAccess: encAccessRestricted,
	}
	return C.ScopeRef{_handle: universe.Add(scopeRestricted)}
}
