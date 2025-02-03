// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/logger"
	"storj.io/storj/shared/mud"
)

// Module registers all the possible components for the satellite instance.
func Module(ball *mud.Ball) {
	logger.Module(ball)
	modular.IdentityModule(ball)
	satellitedb.Module(ball)
	satellite.Module(ball)
	mud.Provide[*modular.MonkitReport](ball, modular.NewMonkitReport)
}
