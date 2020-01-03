// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"fmt"

	"time"

	"storj.io/common/memory"
	"storj.io/storj/lib/uplink"
)

var universe = newHandles()

//export internal_UniverseIsEmpty
// internal_UniverseIsEmpty returns true if nothing is stored in the global map.
func internal_UniverseIsEmpty() bool {
	return universe.Empty()
}

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
func new_uplink(cfg C.UplinkConfig, tempDir *C.char, cerr **C.char) C.UplinkRef {
	scope := rootScope(C.GoString(tempDir))

	libcfg := &uplink.Config{} // TODO: figure out a better name
	// TODO: V3-2302, add a way to support logging
	libcfg.Volatile.TLS.SkipPeerCAWhitelist = cfg.Volatile.tls.skip_peer_ca_whitelist == C.bool(true)
	libcfg.Volatile.TLS.PeerCAWhitelistPath = C.GoString(cfg.Volatile.tls.peer_ca_whitelist_path)
	libcfg.Volatile.PeerIDVersion = C.GoString(cfg.Volatile.peer_id_version)
	libcfg.Volatile.MaxInlineSize = memory.Size(cfg.Volatile.max_inline_size)
	libcfg.Volatile.MaxMemory = memory.Size(cfg.Volatile.max_memory)
	libcfg.Volatile.DialTimeout = time.Duration(cfg.Volatile.dial_timeout)
	libcfg.Volatile.UserAgent = C.GoString(cfg.Volatile.user_agent)

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
