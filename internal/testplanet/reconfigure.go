package testplanet

import (
	"go.uber.org/zap"

	"storj.io/storj/bootstrap"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

// Reconfigure allows to change node configurations
type Reconfigure struct {
	NewBootstrapDB func(index int) (bootstrap.DB, error)
	Bootstrap      func(index int, config *bootstrap.Config)

	NewSatelliteDB func(log *zap.Logger, index int) (satellite.DB, error)
	Satellite      func(log *zap.Logger, index int, config *satellite.Config)

	NewStorageNodeDB func(index int) (storagenode.DB, error)
	StorageNode      func(index int, config *storagenode.Config)
}

var DisablePeerCAWhitelist = Reconfigure{
	Bootstrap: func(index int, config *bootstrap.Config) {
		config.Server.UsePeerCAWhitelist = false
	},
	Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
		config.Server.UsePeerCAWhitelist = false
	},
	StorageNode: func(index int, config *storagenode.Config) {
		config.Server.UsePeerCAWhitelist = false
	},
}
