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
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
)

func cmdAPIRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-api",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("Error starting master database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Config.Metainfo.DatabaseURL, metabase.Config{
		ApplicationName:  "satellite-api",
		MinPartSize:      runCfg.Config.Metainfo.MinPartSize,
		MaxNumberOfParts: runCfg.Config.Metainfo.MaxNumberOfParts,
		ServerSideCopy:   runCfg.Config.Metainfo.ServerSideCopy,
	})
	if err != nil {
		return errs.New("Error creating metabase connection on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Config.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	accountingCache, err := live.OpenCache(ctx, log.Named("live-accounting"), runCfg.LiveAccounting)
	if err != nil {
		if !accounting.ErrSystemOrNetError.Has(err) || accountingCache == nil {
			return errs.New("Error instantiating live accounting cache: %w", err)
		}

		log.Warn("Unable to connect to live accounting cache. Verify connection",
			zap.Error(err),
		)
	}
	defer func() {
		err = errs.Combine(err, accountingCache.Close())
	}()

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
	defer func() {
		err = errs.Combine(err, rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx)))
	}()

	peer, err := satellite.NewAPI(log, identity, db, metabaseDB, revocationDB, accountingCache, rollupsWriteCache, &runCfg.Config, version.Build, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry batcher on satellite api", zap.Error(err))
	}

	err = metabaseDB.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed metabase database version check.", zap.Error(err))
		return errs.New("failed metabase version check: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
