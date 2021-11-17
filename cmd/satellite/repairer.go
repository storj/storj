// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/pb"
	"storj.io/common/uuid"
	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/uplink/private/eestream"
)

func cmdRepairerRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-repairer"})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, metabase.Config{
		MinPartSize:      runCfg.Config.Metainfo.MinPartSize,
		MaxNumberOfParts: runCfg.Config.Metainfo.MaxNumberOfParts,
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

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
	defer func() {
		err = errs.Combine(err, rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx)))
	}()

	peer, err := satellite.NewRepairer(
		log,
		identity,
		metabaseDB,
		revocationDB,
		db.RepairQueue(),
		db.Buckets(),
		db.OverlayCache(),
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

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry batcher on repairer", zap.Error(err))
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

	segments, err := parseInput(log, args[0])
	if err != nil {
		log.Error("Failed to retrieve segment info from file", zap.Error(err))
		return err
	}
	for _, segment := range segments {
		orderlimits, err := DownloadPieces(ctx, metabaseDB, peer.Overlay,
			peer.Orders.Service, peer.EcRepairer, segment.StreamID, segment.Position)
		if err != nil {
			log.Debug("Failed to download pieces", zap.Stringer("StreamID", segment.StreamID), zap.Uint64("Position", segment.Position.Encode()), zap.Error(err))
			for _, limit := range orderlimits {
				log.Debug("Order limit", zap.String("", fmt.Sprintf("%#v", limit)))
			}
		}
		log.Debug("Successful download", zap.Stringer("StreamID", segment.StreamID), zap.Uint64("Position", segment.Position.Encode()), zap.Error(err))
	}

	return peer.Close()
}

func DownloadPieces(
	ctx context.Context,
	metabaseDB *metabase.DB,
	overlay *overlay.Service,
	orders *orders.Service,
	ec *repairer.ECRepairer,
	streamID uuid.UUID,
	position metabase.SegmentPosition,
) (_ []*pb.AddressedOrderLimit, err error) {
	segment, err := metabaseDB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: streamID,
		Position: position,
	})
	if err != nil {
		return nil, err
	}

	// We don't verify if the segment is inline or expired on purpose.

	redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
	if err != nil {
		return nil, err
	}

	pieces := segment.Pieces
	missingPieces, err := overlay.GetMissingPieces(ctx, pieces)
	if err != nil {
		return nil, err
	}

	numHealthy := len(pieces) - len(missingPieces)
	if numHealthy < int(segment.Redundancy.RequiredShares) {
		return nil, fmt.Errorf("irreparable segment: %s/%d/%d", streamID, position.Encode(), numHealthy)
	}

	lostPiecesSet := sliceToSet(missingPieces)
	var healthyPieces metabase.Pieces
	for _, piece := range pieces {
		if !lostPiecesSet[piece.Number] {
			healthyPieces = append(healthyPieces, piece)
		}
	}

	getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := orders.CreateGetRepairOrderLimits(ctx, metabase.BucketLocation{}, segment, healthyPieces)
	if err != nil {
		return nil, err
	}

	reader, _, err := ec.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
	if err != nil {

		return getOrderLimits, err
	}
	defer func() { err = errs.Combine(err, reader.Close()) }()

	return getOrderLimits, nil
}

func sliceToSet(slice []uint16) map[uint16]bool {
	set := make(map[uint16]bool, len(slice))
	for _, value := range slice {
		set[value] = true
	}
	return set
}

func parseInput(log *zap.Logger, input string) (segments []SegmentInfo, err error) {
	f, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, f.Close()) }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		data := strings.Split(scanner.Text(), ",")
		if len(data) < 2 {
			continue
		}
		// Parse the access data to ensure it is well formed
		streamID, err := uuid.FromString(data[0])
		if err != nil {
			log.Debug("Failed to parse stream ID", zap.String("input", data[0]), zap.Error(err))
			continue
		}
		p, err := strconv.ParseUint(data[1], 10, 64)
		if err != nil {
			continue
		}

		segments = append(segments, SegmentInfo{StreamID: streamID, Position: metabase.SegmentPositionFromEncoded(p)})
	}

	if err := scanner.Err(); err != nil {
		return segments, err
	}

	return segments, nil
}

type SegmentInfo struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
}
