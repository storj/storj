// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import "unsafe"

// CString exposes C.String for testing
func CString(s string) *C.char { return C.CString(s) }

// CFree exposes C.free for testing
func CFree(ptr unsafe.Pointer) { C.free(ptr) }

// CGoBytes converts ptr of n bytes to Go bytes.
func CGoBytes(ptr unsafe.Pointer, n C.int) []byte {
	return C.GoBytes(ptr, n)
}

// C types
type Cpchar = *C.char
type Cuint = C.uint
type Cint = C.int
type Cuint8_t = C.uint8_t
type Cint64_t = C.int64_t
type Csize_t = C.size_t

//const CEOF = C.EOF

// Ref types
type CAPIKey = C.APIKey
type CUplink = C.Uplink
type CProject = C.Project
type CBucket = C.Bucket
type CBuffer = C.Buffer
type CObject = C.Object

// Struct types
type CUplinkConfig = C.UplinkConfig
type CBucketInfo = C.BucketInfo

//type CObject = C.Object
//type CUploadOptions = C.UploadOptions
//type CBytes = C.Bytes

//export internal_UniverseIsEmpty
// internal_UniverseIsEmpty returns true if nothing is stored in the global map.
func internal_UniverseIsEmpty() bool {
	return universe.Empty()
}
