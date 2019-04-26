// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include "structs.h"
import (
	"C"
	"storj.io/storj/lib/uplink"
)


//export NewUplink
func NewUplink(CCfg C.struct_Config) {
	ctx := context.Background()

	cfg := convertConfigToGo(CCfg)

	uplink.NewUplink(ctx, cfg)


}

func convertConfigToGo(CCfg C.struct_Config) uplink.Config {

	idVersion, err := storj.GetIDVersion(CCfg.IdentityVersion)
	if err != nil {
		return nil
	}

	return uplink.Config {
		Volatile {
			TLS {
				SkipPeerCAWhitelist: CCfg.SkipPeerCAWhitelist,
				PeerCAWhitelistPath: C.GoString(CCfg.PeerCAWhitelistPath),
			},
			IdentityVersion: idVersion,
			PeerIDVersion: C.GoString(CCfg.PeerIDVersion),
			MaxInlineSize: memory.Size(CCfg.MaxInlineSize),
			MaxMemory: memory.Size(CCfg.MaxMemory),
		},
	}

}