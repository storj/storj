// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/common/debug"
	"storj.io/common/peertls/extensions"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/jobq/jobqueue"
	jobqserver "storj.io/storj/satellite/jobq/server"
	sndebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/modular/eventkit"
	"storj.io/storj/shared/modular/logger"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
)

// Module registers all the possible components for the jobq instance.
func Module(ball *mud.Ball) {
	logger.Module(ball)
	modular.IdentityModule(ball)
	tracing.Module(ball)
	eventkit.Module(ball)
	profiler.Module(ball)
	mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
	config.RegisterConfig[debug.Config](ball, "debug")
	sndebug.Module(ball)

	jobqserver.Module(ball)
	jobqueue.Module(ball)

	mud.Provide[*Run](ball, func() *Run {
		return &Run{}
	})
	cli.RegisterSubcommand[*Run](ball, "run", "Run the job queue service")

}
