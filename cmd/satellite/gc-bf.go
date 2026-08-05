// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"time"

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
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/tidbutil"
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
	// Garbage collection depends on this for correctness: without it, a
	// server-side copy interleaved with the scan can hide live pieces from
	// the bloom filters.
	var readTimestamp time.Time
	if safepoint := runCfg.RangedLoop.Safepoint; safepoint.Enabled() {
		if !runCfg.GarbageCollectionBF.RunOnce {
			return errs.New("safepoint requires run-once mode")
		}
		// Every cluster backing the metabase needs its own hold: an unpinned
		// one is scanned without the snapshot the run promises. The clusters
		// are isolated, each with its own PD, so the PD endpoints are labeled
		// with the metabase backend they serve and every backend must be there.
		impls := metabaseDB.Implementations()
		for label, impl := range impls {
			if impl != dbutil.TiDB {
				return errs.New("safepoint is not supported on %v (metabase backend %q)", impl, label)
			}
		}

		holds, holdErr := tidbutil.Hold(ctx, log.Named("safepoint"), tidbutil.SafepointConfig{
			PDEndpoints: safepoint.PDEndpoints,
			ServiceID:   safepoint.ServiceID,
			TTL:         safepoint.TTL,
			CAPath:      safepoint.CACertPath,
			CertPath:    safepoint.CertPath,
			KeyPath:     safepoint.KeyPath,
		})
		if holdErr != nil {
			return errs.New("Error acquiring GC safepoint: %+v", holdErr)
		}
		defer func() {
			// release with a fresh deadline even when the scan context is
			// already cancelled; the TTL remains the backstop if this fails
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			err = errs.Combine(err, holds.Release(releaseCtx))
		}()

		for label := range impls {
			if _, ok := holds[label]; !ok {
				return errs.New("no safepoint PD endpoints configured for metabase backend %q", label)
			}
		}
		for label := range holds {
			if _, ok := impls[label]; !ok {
				return errs.New("safepoint PD endpoints configured for %q, which is not a metabase backend", label)
			}
		}

		// scan at the pinned timestamp; abort the run if any hold is ever lost
		readTimestamp = holds.ReadTime()
		ctx = holds.Context(ctx)
		if safepoint.MaxDuration > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeoutCause(ctx, safepoint.MaxDuration,
				errs.New("scan exceeded safepoint max duration %s", safepoint.MaxDuration))
			defer cancel()
		}

		log.Info("holding TiKV GC safepoints for the scan",
			zap.Time("read_timestamp", readTimestamp),
			zap.Strings("service_ids", holds.ServiceIDs()),
			zap.Duration("ttl", safepoint.TTL))
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
