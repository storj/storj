// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/modular/logger"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode"
)

// Module registers all the possible components for the storagenode instance.
func Module(ball *mud.Ball) {
	logger.Module(ball)
	modular.IdentityModule(ball)
	storagenode.Module(ball)
	mud.Provide[*Setup](ball, NewSetup)
	cli.RegisterSubcommand[*Setup](ball, "setup", "setup storagenode configuration")
}
