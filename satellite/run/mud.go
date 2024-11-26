// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/logger"
)

// CreateModule registers all the possible components for the satellite instance.
func CreateModule() *mud.Ball {
	ball := &mud.Ball{}
	logger.Module(ball)
	modular.IdentityModule(ball)
	satellitedb.Module(ball)
	satellite.Module(ball)
	return ball
}
