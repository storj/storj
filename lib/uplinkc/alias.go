// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import "unsafe"

// CPChar exposing C for testing
type CPChar = *C.char

// CUplinkConfig exposing C for testing
type CUplinkConfig = C.UplinkConfig

// CString exposes C.String for testing
func CString(s string) *C.char { return C.CString(s) }

// CFree exposes C.free for testing
func CFree(ptr unsafe.Pointer) { C.free(ptr) }

//export internal_UniverseIsEmpty
// internal_UniverseIsEmpty returns true if nothing is stored in the global map.
func internal_UniverseIsEmpty() bool {
	return universe.Empty()
}
