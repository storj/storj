// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/process/eventkitbq"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb"
)

func cmdRangedLoopRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-rangedloop"})
	if err != nil {
		return errs.New("Error starting master database on satellite rangedloop: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, runCfg.Metainfo.Metabase("satellite-rangedloop"))
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		return errs.New("Error loading identity: %+v", err)
	}

	var repairQueue queue.RepairQueue
	if !runCfg.JobQueue.ServerNodeURL.IsZero() {
		repairQueue, err = jobq.OpenJobQueue(ctx, identity, runCfg.JobQueue)
		if err != nil {
			return errs.New("Error opening repair queue: %+v", err)
		}
	} else {
		repairQueue = db.RepairQueue()
	}

	peer, err := satellite.NewRangedLoop(log, db, metabaseDB, repairQueue, &runCfg.Config, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), eventkitbq.BQDestination); err != nil {
		log.Warn("Failed to initialize telemetry on satellite rangedloop", zap.Error(err))
	}

	if err := checkDBVersions(ctx, log, runCfg, db, metabaseDB); err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
