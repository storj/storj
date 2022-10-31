// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/uuid"
	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
)

func cmdRepairSegment(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	streamID, err := uuid.FromString(args[0])
	if err != nil {
		return errs.New("invalid stream-id (should be in UUID form): %w", err)
	}
	streamPosition, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return errs.New("stream position must be a number: %w", err)
	}

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-pieces-fetcher"})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-repair-segment"))
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

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
	defer func() {
		err = errs.Combine(err, rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx)))
	}()

	// TODO: disable reputation and containment system.

	peer, err := satellite.NewRepairer(
		log,
		identity,
		metabaseDB,
		revocationDB,
		db.RepairQueue(),
		db.Buckets(),
		db.OverlayCache(),
		db.NodeEvents(),
		db.Reputation(),
		db.Containment(),
		rollupsWriteCache,
		version.Build,
		&runCfg.Config,
		process.AtomicLevel(cmd),
	)
	if err != nil {
		return err
	}

	segmentInfo, err := metabaseDB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: streamID,
		Position: metabase.SegmentPositionFromEncoded(streamPosition),
	})
	if err != nil {
		return err
	}

	pieceInfos, err := peer.SegmentRepairer.AdminFetchPieces(ctx, &segmentInfo, "")
	if err != nil {
		return err
	}

	for pieceNum, pieceInfo := range pieceInfos {
		if pieceInfo.GetLimit == nil {
			continue
		}
		log := log.With(zap.Int("piece-index", pieceNum))

		if err := pieceInfo.FetchError; err != nil {
			log.Error("failed to fetch", zap.Error(err))
			continue
		}
		if pieceInfo.Reader == nil {
			log.Error("piece reader missing")
			continue
		}

		log.Info("piece loaded")

		// TODO: maybe read into memory?
		// TODO: do we need to verify hash?

		if err := pieceInfo.Reader.Close(); err != nil {
			log.Error("could not close piece reader", zap.Error(err))
			continue
		}
	}

	// TODO: reconstruct and upload pieces

	return nil
}
