// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
	trustmud "storj.io/storj/satellite/trust/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/modular/logger"
	"storj.io/storj/shared/mud"
)

// Module registers all the possible components for the satellite instance.
func Module(ball *mud.Ball) {
	logger.Module(ball)
	modular.IdentityModule(ball)

	// defining the databases here, and not in satellite.Module, as mudplanet would like to use different options
	satellitedb.Module(ball)
	mud.Provide[*metabase.DB](ball, metabase.OpenDatabaseWithMigration)

	satellite.Module(ball)
	trustmud.Module(ball)

	mud.Provide[*modular.MonkitReport](ball, modular.NewMonkitReport)

	mud.Provide[*Auditor](ball, func() *Auditor {
		return &Auditor{}
	})
	cli.RegisterSubcommand[*Auditor](ball, "auditor", "run the auditor service")
	mud.Provide[*Repair](ball, func() *Repair {
		return &Repair{}
	})
	cli.RegisterSubcommand[*Repair](ball, "repair", "run the repair worker service")
	mud.Provide[*ChangeStream](ball, func() *ChangeStream {
		return &ChangeStream{}
	})
	cli.RegisterSubcommand[*ChangeStream](ball, "change-stream", "run the Spanner change stream processor service")
}
