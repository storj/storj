// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/private/context2"
	"storj.io/storj/private/version"
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
		zap.S().Fatal(err)
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
		return errs.New("Error creating metainfo database: %+v", err)
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
		db.Orders(),
		rollupsWriteCache,
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
		zap.S().Warn("Failed to initialize telemetry batcher on repairer: ", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		zap.S().Fatal("failed satellite database version check: ", err)
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
