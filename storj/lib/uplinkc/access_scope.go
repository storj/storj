// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"

	libuplink "storj.io/storj/lib/uplink"
)

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

//export free_scope
// free_scope frees an scope
func free_scope(scopeHandle C.ScopeRef) {
	universe.Del(scopeHandle._handle)
}
