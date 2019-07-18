// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"fmt"
	"unsafe"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

//export new_encryption_access
// new_encryption_access creates an encryption access context
func new_encryption_access(cerr **C.char) C.EncryptionAccessRef {
	return C.EncryptionAccessRef {
		_handle:  universe.Add(uplink.NewEncryptionAccess()),
	}
}

//export new_encryption_access_with_default_key
// new_encryption_access creates an encryption access context with a default key set.
func new_encryption_access_with_default_key(key *C.uint8_t) C.EncryptionAccessRef {
	goKey, cKey := storj.Key{}, (*storj.Key)(unsafe.Pointer(key))
	copy(goKey[:], cKey[:])

	return C.EncryptionAccessRef {
		_handle:  universe.Add(uplink.NewEncryptionAccessWithDefaultKey(goKey)),
	}
}

//export set_default_key
// set_default_key sets the default key for the encryption access context.
func set_default_key(encAccessRef C.EncryptionAccessRef, key *C.uint8_t, cerr **C.char) {
	encAccess, ok := universe.Get(encAccessRef._handle).(*uplink.EncryptionAccess)
	if !ok {
		*cerr = C.CString("invalid encryption access")
		return
	}

	goKey, cKey := storj.Key{}, (*storj.Key)(unsafe.Pointer(key))
	copy(goKey[:], cKey[:])

	encAccess.SetDefaultKey(goKey)
}

//export serialize_encryption_access
// serialize_encryption_access turns an encryption access into base58.
func serialize_encryption_access(encAccessRef C.EncryptionAccessRef, cerr **C.char) *C.char {
	encAccess, ok := universe.Get(encAccessRef._handle).(*uplink.EncryptionAccess)
	if !ok {
		*cerr = C.CString("invalid encryption access")
		return C.CString("")
	}

	encAccessStr, err := encAccess.Serialize()
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.CString("")
	}

	return C.CString(encAccessStr)
}