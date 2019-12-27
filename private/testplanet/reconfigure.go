// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testplanet

import (
	"time"

	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storagenode"
)

// Reconfigure allows to change node configurations
type Reconfigure struct {
	NewSatelliteDB        func(log *zap.Logger, index int) (satellite.DB, error)
	NewSatellitePointerDB func(log *zap.Logger, index int) (metainfo.PointerDB, error)
	Satellite             func(log *zap.Logger, index int, config *satellite.Config)
	ReferralManagerServer func(log *zap.Logger) pb.ReferralManagerServer

	NewStorageNodeDB func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error)
	StorageNode      func(index int, config *storagenode.Config)
	UniqueIPCount    int

	Identities func(log *zap.Logger, version storj.IDVersion) *testidentity.Identities
}

// DisablePeerCAWhitelist returns a `Reconfigure` that sets `UsePeerCAWhitelist` for
// all node types that use kademlia.
var DisablePeerCAWhitelist = Reconfigure{
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Server.UsePeerCAWhitelist = false
	},
	StorageNode: func(index int, config *storagenode.Config) {
		config.Server.UsePeerCAWhitelist = false
	},
}

// ShortenOnlineWindow returns a `Reconfigure` that sets the NodeSelection
// OnlineWindow to 1 second, meaning a connection failure leads to marking the nodes as offline
var ShortenOnlineWindow = Reconfigure{
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Overlay.Node.OnlineWindow = 1 * time.Second
	},
}
