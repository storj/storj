// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"fmt"
	"unsafe"

	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

//export new_encryption_access
// new_encryption_access creates an encryption access context
func new_encryption_access(cerr **C.char) C.EncryptionAccessRef {
	return C.EncryptionAccessRef{
		_handle: universe.Add(libuplink.NewEncryptionAccess()),
	}
}

//export new_encryption_access_with_default_key
// new_encryption_access creates an encryption access context with a default key set.
func new_encryption_access_with_default_key(key *C.uint8_t) C.EncryptionAccessRef {
	goKey, cKey := storj.Key{}, (*storj.Key)(unsafe.Pointer(key))
	copy(goKey[:], cKey[:])

	return C.EncryptionAccessRef{
		_handle: universe.Add(libuplink.NewEncryptionAccessWithDefaultKey(goKey)),
	}
}

//export set_default_key
// set_default_key sets the default key for the encryption access context.
func set_default_key(encAccessRef C.EncryptionAccessRef, key *C.uint8_t, cerr **C.char) {
	encAccess, ok := universe.Get(encAccessRef._handle).(*libuplink.EncryptionAccess)
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
	encAccess, ok := universe.Get(encAccessRef._handle).(*libuplink.EncryptionAccess)
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

//export parse_encryption_access
// parse_encryption_access parses a base58 serialized encryption access into a working one.
func parse_encryption_access(encAccessStr *C.char, cerr **C.char) C.EncryptionAccessRef {
	encAccess, err := libuplink.ParseEncryptionAccess(C.GoString(encAccessStr))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.EncryptionAccessRef{}
	}
	return C.EncryptionAccessRef{_handle: universe.Add(encAccess)}
}

//export free_encryption_access
func free_encryption_access(encAccessRef C.EncryptionAccessRef) {
	universe.Del(encAccessRef._handle)
}
