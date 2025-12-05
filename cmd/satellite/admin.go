// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/process/eventkitbq"
	"storj.io/common/version"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	_ "storj.io/storj/satellite/admin/back-office/ui" // embed ui
	_ "storj.io/storj/satellite/admin/ui"             // embed ui
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func cmdAdminRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:   "satellite-admin",
		APIKeysLRUOptions: runCfg.APIKeysLRUOptions(),
	})
	if err != nil {
		return errs.New("Error starting master database on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Config.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-admin"))
	if err != nil {
		return errs.New("Error creating metabase connection on satellite api: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
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

	peer, err := satellite.NewAdmin(log, identity, db, metabaseDB, accountingCache, version.Build, &runCfg.Config, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), eventkitbq.BQDestination); err != nil {
		log.Warn("Failed to initialize telemetry batcher on satellite admin", zap.Error(err))
	}

	if err := checkDBVersions(ctx, log, runCfg, db, metabaseDB); err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
