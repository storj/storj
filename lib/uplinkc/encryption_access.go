// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import "storj.io/storj/lib/uplink"

//export new_encryption_access
// new_encryption_access creates an encryption access context
func new_encryption_access(cerr **C.char) *C.char {
	enc_access := uplink.NewEncryptionAccess()
	enc_access_str, err := enc_access.Serialize()
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.CString("")
	}
	return C.CString(enc_access_str)
}
