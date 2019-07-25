// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"

	"storj.io/storj/lib/uplink"
)

// Uplink is a scoped uplink.Uplink.
type Uplink struct {
	scope
	*uplink.Uplink
}

//export new_uplink
// new_uplink creates the uplink with the specified configuration and returns
// an error in cerr, when there is one.
//
// Caller must call close_uplink to close associated resources.
func new_uplink(cfg C.UplinkConfig, cerr **C.char) C.UplinkRef {
	scope := rootScope("") // TODO: pass in as argument

	libcfg := &uplink.Config{} // TODO: figure out a better name
	libcfg.Volatile.TLS.SkipPeerCAWhitelist = cfg.Volatile.tls.skip_peer_ca_whitelist == C.bool(true)

	lib, err := uplink.NewUplink(scope.ctx, libcfg)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.UplinkRef{}
	}

	return C.UplinkRef{universe.Add(&Uplink{scope, lib})}
}

//export close_uplink
// close_uplink closes and frees the resources associated with uplink
func close_uplink(uplinkHandle C.UplinkRef, cerr **C.char) {
	uplink, ok := universe.Get(uplinkHandle._handle).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(uplinkHandle._handle)
	defer uplink.cancel()

	if err := uplink.Close(); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}
