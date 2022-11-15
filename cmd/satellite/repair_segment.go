// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/context2"
	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/process"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/uplink/private/eestream"
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

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-segment-repairer"})
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

	config := runCfg

	tlsOptions, err := tlsopts.NewOptions(identity, config.Server.Config, revocationDB)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	// mail service is nil
	overlay, err := overlay.NewService(log.Named("overlay"), db.OverlayCache(), db.NodeEvents(), nil, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
	if err != nil {
		return err
	}

	orders, err := orders.NewService(
		log.Named("orders"),
		signing.SignerFromFullIdentity(identity),
		overlay,
		db.Orders(),
		config.Orders,
	)
	if err != nil {
		return err
	}

	ecRepairer := repairer.NewECRepairer(
		log.Named("ec-repair"),
		dialer,
		signing.SigneeFromPeerIdentity(identity.PeerIdentity()),
		config.Repairer.DownloadTimeout,
		true) // force inmemory download of pieces

	segmentRepairer := repairer.NewSegmentRepairer(
		log.Named("segment-repair"),
		metabaseDB,
		orders,
		overlay,
		nil, // TODO add noop version
		ecRepairer,
		config.Checker.RepairOverrides,
		config.Repairer.Timeout,
		config.Repairer.MaxExcessRateOptimalThreshold,
	)

	// TODO reorganize to avoid using peer.

	peer := &satellite.Repairer{}
	peer.Overlay = overlay
	peer.Orders.Service = orders
	peer.EcRepairer = ecRepairer
	peer.SegmentRepairer = segmentRepairer

	cancelCtx, cancel := context.WithCancel(ctx)
	group := errgroup.Group{}
	group.Go(func() error {
		return peer.Overlay.UploadSelectionCache.Run(cancelCtx)
	})
	defer func() {
		cancel()
		err := group.Wait()
		if err != nil {
			log.Error("upload cache error", zap.Error(err))
		}
	}()

	segment, err := metabaseDB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
		StreamID: streamID,
		Position: metabase.SegmentPositionFromEncoded(streamPosition),
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			printOutput(segment.StreamID, segment.Position.Encode(), "segment not found in metabase db", 0, 0)
			return nil
		}
		log.Error("unknown error when getting segment metadata",
			zap.Stringer("stream-id", streamID),
			zap.Uint64("position", streamPosition),
			zap.Error(err))
		printOutput(segment.StreamID, segment.Position.Encode(), "internal", 0, 0)
		return nil
	}

	return repairSegment(ctx, log, peer, metabaseDB, segment)
}

// repairSegment will repair selected segment no matter if it's healthy or not.
//
// Logic for this method is:
// * download whole segment into memory, use all available pieces
// * reupload segment into new nodes
// * replace segment.Pieces field with just new nodes.
func repairSegment(ctx context.Context, log *zap.Logger, peer *satellite.Repairer, metabaseDB *metabase.DB, segment metabase.Segment) error {
	log = log.With(zap.Stringer("stream-id", segment.StreamID), zap.Uint64("position", segment.Position.Encode()))
	segmentData, failedDownloads, err := downloadSegment(ctx, log, peer, metabaseDB, segment)
	if err != nil {
		log.Error("download failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "download failed", len(segment.Pieces), failedDownloads)
		return nil
	}

	if err := reuploadSegment(ctx, log, peer, metabaseDB, segment, segmentData); err != nil {
		log.Error("upload failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "upload failed", len(segment.Pieces), failedDownloads)
		return nil
	}

	printOutput(segment.StreamID, segment.Position.Encode(), "successful", len(segment.Pieces), failedDownloads)
	return nil
}

func reuploadSegment(ctx context.Context, log *zap.Logger, peer *satellite.Repairer, metabaseDB *metabase.DB, segment metabase.Segment, segmentData []byte) error {
	excludeNodeIDs := make([]storj.NodeID, 0, len(segment.Pieces))
	for _, piece := range segment.Pieces {
		excludeNodeIDs = append(excludeNodeIDs, piece.StorageNode)
	}

	redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
	if err != nil {
		return err
	}

	request := overlay.FindStorageNodesRequest{
		RequestedCount: redundancy.OptimalThreshold(),
		ExcludedIDs:    excludeNodeIDs,
		Placement:      segment.Placement,
	}

	newNodes, err := peer.Overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return err
	}

	if len(newNodes) < redundancy.RepairThreshold() {
		return errs.New("not enough new nodes were found for repair: min %v got %v", redundancy.RepairThreshold(), len(newNodes))
	}

	optimalThresholdMultiplier := float64(1) // is this value fine?
	numHealthyInExcludedCountries := 0
	putLimits, putPrivateKey, err := peer.Orders.Service.CreatePutRepairOrderLimits(ctx, metabase.BucketLocation{}, segment,
		make([]*pb.AddressedOrderLimit, len(newNodes)), newNodes, optimalThresholdMultiplier, numHealthyInExcludedCountries)
	if err != nil {
		return errs.New("could not create PUT_REPAIR order limits: %w", err)
	}

	timeout := 5 * time.Minute
	successfulNeeded := redundancy.OptimalThreshold()
	successful, _, err := peer.EcRepairer.Repair(ctx, putLimits, putPrivateKey, redundancy, bytes.NewReader(segmentData),
		timeout, successfulNeeded)
	if err != nil {
		return err
	}

	var repairedPieces metabase.Pieces
	for i, node := range successful {
		if node == nil {
			continue
		}
		repairedPieces = append(repairedPieces, metabase.Piece{
			Number:      uint16(i),
			StorageNode: node.Id,
		})
	}

	if len(repairedPieces) < redundancy.RepairThreshold() {
		return errs.New("not enough pieces were uploaded during repair: min %v got %v", redundancy.RepairThreshold(), len(repairedPieces))
	}

	// UpdateSegmentPieces is doing compare and swap
	return metabaseDB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segment.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     repairedPieces,

		NewRepairedAt: time.Now(),
	})
}

func downloadSegment(ctx context.Context, log *zap.Logger, peer *satellite.Repairer, metabaseDB *metabase.DB, segment metabase.Segment) ([]byte, int, error) {
	// AdminFetchPieces downloads all pieces for specified segment and returns readers, readers data is kept on disk or inmemory
	pieceInfos, err := peer.SegmentRepairer.AdminFetchPieces(ctx, &segment, "")
	if err != nil {
		return nil, 0, err
	}

	numberOfOtherFailures := 0
	numberOfFileNotFound := 0
	numberOfOffline := 0
	pieceReaders := make(map[int]io.ReadCloser, len(pieceInfos))
	for pieceNum, pieceInfo := range pieceInfos {
		if pieceInfo.GetLimit == nil {
			continue
		}

		log := log.With(zap.Int("piece num", pieceNum))

		var dnsErr *net.DNSError
		var opError *net.OpError
		if err := pieceInfo.FetchError; err != nil {
			if errs2.IsRPC(err, rpcstatus.NotFound) {
				numberOfFileNotFound++
			} else if errors.As(err, &dnsErr) || errors.As(err, &opError) {
				numberOfOffline++
			} else {
				numberOfOtherFailures++
			}

			log.Error("unable to fetch piece", zap.Error(pieceInfo.FetchError))
			continue
		}
		if pieceInfo.Reader == nil {
			log.Error("piece reader is empty")
			continue
		}

		pieceReaders[pieceNum] = pieceInfo.Reader
	}

	log.Info("download summary",
		zap.Int("number of pieces", len(segment.Pieces)), zap.Int("pieces downloaded", len(pieceReaders)),
		zap.Int("file not found", numberOfFileNotFound), zap.Int("offline nodes", numberOfOffline),
		zap.Int("other errors", numberOfOtherFailures),
	)

	failedDownloads := numberOfFileNotFound + numberOfOtherFailures

	redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
	if err != nil {
		return nil, failedDownloads, errs.New("invalid redundancy strategy: %w", err)
	}

	if len(pieceReaders) < redundancy.RequiredCount() {
		return nil, failedDownloads, errs.New("not enough pieces to reconstruct the segment, pieces: %d required: %d",
			len(pieceReaders), redundancy.RequiredCount())
	}

	fec, err := infectious.NewFEC(redundancy.RequiredCount(), redundancy.TotalCount())
	if err != nil {
		return nil, failedDownloads, err
	}

	esScheme := eestream.NewUnsafeRSScheme(fec, redundancy.ErasureShareSize())
	pieceSize := eestream.CalcPieceSize(int64(segment.EncryptedSize), redundancy)
	expectedSize := pieceSize * int64(redundancy.RequiredCount())

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	segmentReader := eestream.DecodeReaders2(ctx, cancel, pieceReaders, esScheme, expectedSize, 0, false)
	data, err := io.ReadAll(segmentReader)
	return data, failedDownloads, err
}

// printOutput prints result to standard output in a way to be able to combine
// single results into single csv file.
func printOutput(streamID uuid.UUID, position uint64, result string, numberOfPieces, failedDownloads int) {
	fmt.Printf("%s,%d,%s,%d,%d\n", streamID, position, result, numberOfPieces, failedDownloads)
}
