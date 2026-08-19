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
	"storj.io/common/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/satellitedb"
)

func cmdGCBloomFilterRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-gc-bloomfilter"})
	if err != nil {
		return errs.New("Error starting master database on satellite GC: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-gc-bloomfilter"))
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database GC: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	// Pin a TiKV GC safepoint so the whole scan reads one consistent snapshot.
	if runCfg.RangedLoop.Safepoint.Enabled() && !runCfg.GarbageCollectionBF.RunOnce {
		return errs.New("safepoint requires run-once mode")
	}
	safepoint, err := rangedloop.HoldSafepoint(ctx, log, metabaseDB, runCfg.RangedLoop.Safepoint)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, safepoint.Close())
	}()

	// scan at the pinned timestamp; abort the run if any hold is ever lost
	ctx, cancel := safepoint.Context(ctx)
	defer cancel()
	readTimestamp := safepoint.ReadTime()

	// the modular path enforces this off the bloom filter observer; here
	// NewGarbageCollectionBF builds the splitter, and testplanet builds that
	// peer too, so the command checks what its splitter is going to read
	splitter := rangedloop.NewMetabaseRangeSplitterWithReadTimestamp(log, metabaseDB, runCfg.RangedLoop, readTimestamp)
	if !splitter.ReadsSnapshot() {
		return rangedloop.ErrNoSnapshot
	}

	peer, err := satellite.NewGarbageCollectionBF(log, db, metabaseDB, revocationDB, version.Build, &runCfg.Config, process.AtomicLevel(cmd), readTimestamp)
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), eventkitbq.BQDestination); err != nil {
		log.Warn("Failed to initialize telemetry batcher on satellite GC", zap.Error(err))
	}

	if err := checkDBVersions(ctx, log, runCfg, db, metabaseDB); err != nil {
		return err
	}

	runError := peer.Run(ctx)

	if err := process.Report(ctx); err != nil {
		log.Warn("could not send telemetry", zap.Error(err))
	}

	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
