// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testplanet

import (
	"time"

	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/multinode"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storagenode"
	"storj.io/storj/versioncontrol"
)

// Reconfigure allows to change node configurations.
type Reconfigure struct {
	SatelliteDB               func(log *zap.Logger, index int, db satellite.DB) (satellite.DB, error)
	SatelliteDBOptions        func(log *zap.Logger, index int, options *satellitedb.Options)
	SatelliteMetabaseDB       func(log *zap.Logger, index int, db *metabase.DB) (*metabase.DB, error)
	SatelliteMetabaseDBConfig func(log *zap.Logger, index int, config *metabase.Config)
	Satellite                 func(log *zap.Logger, index int, config *satellite.Config)
	Uplink                    func(log *zap.Logger, index int, config *UplinkConfig)

	StorageNodeDB func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error)
	StorageNode   func(index int, config *storagenode.Config)
	UniqueIPCount int

	VersionControl func(config *versioncontrol.Config)

	Identities func(log *zap.Logger, version storj.IDVersion) *testidentity.Identities

	MultinodeDB func(index int, db multinode.DB, log *zap.Logger) (multinode.DB, error)
	Multinode   func(index int, config *multinode.Config)
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
// OnlineWindow to 1 second, meaning a connection failure leads to marking the nodes as offline.
var ShortenOnlineWindow = Reconfigure{
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Overlay.Node.OnlineWindow = 1 * time.Second
	},
}

// Combine combines satellite reconfigure functions.
var Combine = func(elements ...func(log *zap.Logger, index int, config *satellite.Config)) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		for _, f := range elements {
			f(log, index, config)
		}
	}
}

// ReconfigureRS returns function to change satellite redundancy scheme values.
var ReconfigureRS = func(minThreshold, repairThreshold, successThreshold, totalThreshold int) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Metainfo.RS.Min = minThreshold
		config.Metainfo.RS.Repair = repairThreshold
		config.Metainfo.RS.Success = successThreshold
		config.Metainfo.RS.Total = totalThreshold
	}
}

// RepairExcludedCountryCodes returns function to change satellite repair excluded country codes.
var RepairExcludedCountryCodes = func(repairExcludedCountryCodes []string) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Overlay.RepairExcludedCountryCodes = repairExcludedCountryCodes
	}
}

// UploadExcludedCountryCodes returns function to change satellite upload excluded country codes.
var UploadExcludedCountryCodes = func(uploadExcludedCountryCodes []string) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Overlay.Node.UploadExcludedCountryCodes = uploadExcludedCountryCodes
	}
}

// MaxSegmentSize returns function to change satellite max segment size value.
var MaxSegmentSize = func(maxSegmentSize memory.Size) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Metainfo.MaxSegmentSize = maxSegmentSize
	}
}

// MaxMetadataSize returns function to change satellite max metadata size value.
var MaxMetadataSize = func(maxMetadataSize memory.Size) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Metainfo.MaxMetadataSize = maxMetadataSize
	}
}

// MaxObjectKeyLength returns function to change satellite max object key length value.
var MaxObjectKeyLength = func(maxObjectKeyLength int) func(log *zap.Logger, index int, config *satellite.Config) {
	return func(log *zap.Logger, index int, config *satellite.Config) {
		config.Metainfo.MaxEncryptedObjectKeyLength = maxObjectKeyLength
	}
}

// DisableTCP prevents both satellite and storagenode being able to accept new
// tcp connections.
var DisableTCP = Reconfigure{
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Server.DisableTCP = true
	},
	StorageNode: func(index int, config *storagenode.Config) {
		config.Server.DisableTCP = true
	},
}

// DisableQUIC prevents both satellite and storagenode being able to accept new
// quic connections.
var DisableQUIC = Reconfigure{
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Server.DisableQUIC = true
	},
	StorageNode: func(index int, config *storagenode.Config) {
		config.Server.DisableQUIC = true
	},
}

// SatelliteDBDisableCaches helper function to disable caches in satellite db.
func SatelliteDBDisableCaches(log *zap.Logger, index int, options *satellitedb.Options) {
	options.APIKeysLRUOptions.Capacity = 0
	options.RevocationLRUOptions.Capacity = 0
}
