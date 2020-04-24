// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
)

func cmdRepairerRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Fatal("Failed to load identity.", zap.Error(err))
	}

	db, err := satellitedb.New(log.Named("db"), runCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	pointerDB, err := metainfo.NewStore(log.Named("pointerdb"), runCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.New("Error creating metainfo database connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, pointerDB.Close())
	}()

	revocationDB, err := revocation.NewDBFromCfg(runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
	defer func() {
		err = errs.Combine(err, rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx)))
	}()

	peer, err := satellite.NewRepairer(
		log,
		identity,
		pointerDB,
		revocationDB,
		db.RepairQueue(),
		db.Buckets(),
		db.OverlayCache(),
		rollupsWriteCache,
		db.Irreparable(),
		version.Build,
		&runCfg.Config,
	)
	if err != nil {
		return err
	}

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry batcher on repairer", zap.Error(err))
	}

	err = pointerDB.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for metainfo database: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		log.Fatal("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
