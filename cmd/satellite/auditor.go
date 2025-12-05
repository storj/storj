// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/process"
	"storj.io/common/process/eventkitbq"
	"storj.io/common/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func cmdAuditorRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-auditor"})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, metabase.Config{
		ApplicationName:  "satellite-auditor",
		MinPartSize:      runCfg.Config.Metainfo.MinPartSize,
		MaxNumberOfParts: runCfg.Config.Metainfo.MaxNumberOfParts,
		ServerSideCopy:   runCfg.Config.Metainfo.ServerSideCopy,
	})
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := satellite.NewAuditor(
		log,
		identity,
		metabaseDB,
		revocationDB,
		db.VerifyQueue(),
		db.ReverifyQueue(),
		db.OverlayCache(),
		db.NodeEvents(),
		db.Reputation(),
		db.Containment(),
		version.Build,
		&runCfg.Config,
		process.AtomicLevel(cmd),
	)
	if err != nil {
		return err
	}

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), eventkitbq.BQDestination); err != nil {
		log.Warn("Failed to initialize telemetry batcher on auditor", zap.Error(err))
	}

	if err := checkDBVersions(ctx, log, runCfg, db, metabaseDB); err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs2.IgnoreCanceled(errs.Combine(runError, closeError))
}
