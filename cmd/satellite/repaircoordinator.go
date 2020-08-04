// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/common/errs2"

	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb"
)

func cmdRepairCoordinatorRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		APIKeysLRUOptions: runCfg.APIKeysLRUOptions(),
		ApplicationName:   "satellite-repaircoordinator",
	})
	if err != nil {
		return errs.New("Error starting master database on satellite repair coordinator: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	pointerDB, err := metainfo.OpenStore(ctx, log.Named("pointerdb"), runCfg.Config.Metainfo.DatabaseURL, "satellite-repaircoordinator")
	if err != nil {
		return errs.New("Error creating metainfodb connection on satellite repair coordinator: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, pointerDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Config.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database on satellite repair coordinator: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := satellite.NewRepairCoordinator(log, identity, db, pointerDB, revocationDB, db.Buckets(), db.OverlayCache(), version.Build, &runCfg.Config, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry batcher on satellite repair coordinator", zap.Error(err))
	}

	err = pointerDB.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating metainfodb tables on satellite repair coordinator: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs2.IgnoreCanceled(errs.Combine(runError, closeError))
}
