// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import "storj.io/storj/pkg/macaroon"

//export new_caveat
// new_caveat returns a Caveat with a random generated nonce.
func new_caveat(cerr **C.char) C.Caveat {
	caveat, err := macaroon.NewCaveat()
	if err != nil {
		*cerr = C.CString(err.Error())
		return nil
	}
	return newCaveat(&caveat)
}

//export free_caveat
// free_caveat frees a caveat
func free_caveat(caveat C.Caveat) {
	universe.Del(caveat.allowed_paths._handle)
	universe.Del(caveat.nonce._handle)
}
