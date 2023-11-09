// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

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

type segment struct {
	StreamID uuid.UUID
	Position uint64
}

func cmdRepairSegment(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	segments, err := collectInputSegments(args)
	if err != nil {
		return err
	}

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-repair-segment"})
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

	config := runCfg

	tlsOptions, err := tlsopts.NewOptions(identity, config.Server.Config, revocationDB)
	if err != nil {
		return err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)

	placement, err := config.Placement.Parse()
	if err != nil {
		return err
	}

	overlayService, err := overlay.NewService(log.Named("overlay"), db.OverlayCache(), db.NodeEvents(), placement.CreateFilters, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
	if err != nil {
		return err
	}

	orders, err := orders.NewService(
		log.Named("orders"),
		signing.SignerFromFullIdentity(identity),
		overlayService,
		orders.NewNoopDB(),
		placement.CreateFilters,
		config.Orders,
	)
	if err != nil {
		return err
	}

	ecRepairer := repairer.NewECRepairer(
		log.Named("ec-repair"),
		dialer,
		signing.SigneeFromPeerIdentity(identity.PeerIdentity()),
		config.Repairer.DialTimeout,
		config.Repairer.DownloadTimeout,
		true) // force inmemory download of pieces

	segmentRepairer := repairer.NewSegmentRepairer(
		log.Named("segment-repair"),
		metabaseDB,
		orders,
		overlayService,
		nil, // TODO add noop version
		ecRepairer,
		placement.CreateFilters,
		config.Checker.RepairOverrides,
		config.Repairer,
	)

	// TODO reorganize to avoid using peer.

	peer := &satellite.Repairer{}
	peer.Overlay = overlayService
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

	for _, segment := range segments {
		segment, err := metabaseDB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: segment.StreamID,
			Position: metabase.SegmentPositionFromEncoded(segment.Position),
		})
		if err != nil {
			if metabase.ErrSegmentNotFound.Has(err) {
				printOutput(segment.StreamID, segment.Position.Encode(), "segment not found in metabase db", 0, 0)
			} else {
				log.Error("unknown error when getting segment metadata",
					zap.Stringer("stream-id", segment.StreamID),
					zap.Uint64("position", segment.Position.Encode()),
					zap.Error(err))
				printOutput(segment.StreamID, segment.Position.Encode(), "internal", 0, 0)
			}
			continue
		}
		repairSegment(ctx, log, peer, metabaseDB, segment)
	}
	return nil
}

func collectInputSegments(args []string) (segments []segment, err error) {
	convert := func(streamIDString, positionString string) (segment, error) {
		streamID, err := uuid.FromString(streamIDString)
		if err != nil {
			return segment{}, errs.New("invalid stream-id (should be in UUID form): %w", err)
		}
		streamPosition, err := strconv.ParseUint(positionString, 10, 64)
		if err != nil {
			return segment{}, errs.New("stream position must be a number: %w", err)
		}
		return segment{
			StreamID: streamID,
			Position: streamPosition,
		}, nil
	}

	if len(args) == 1 {
		csvFile, err := os.Open(args[0])
		if err != nil {
			return nil, err
		}
		defer func() {
			err = errs.Combine(err, csvFile.Close())
		}()

		csvReader := csv.NewReader(csvFile)
		allEntries, err := csvReader.ReadAll()
		if err != nil {
			return nil, err
		}
		if len(allEntries) > 1 {
			// ignore first line with headers
			for _, entry := range allEntries[1:] {
				segment, err := convert(entry[0], entry[1])
				if err != nil {
					return nil, err
				}
				segments = append(segments, segment)
			}
		}
	} else {
		segment, err := convert(args[0], args[1])
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	return segments, nil
}

// repairSegment will repair selected segment no matter if it's healthy or not.
//
// Logic for this method is:
// * download whole segment into memory, use all available pieces
// * reupload segment into new nodes
// * replace segment.Pieces field with just new nodes.
func repairSegment(ctx context.Context, log *zap.Logger, peer *satellite.Repairer, metabaseDB *metabase.DB, segment metabase.Segment) {
	log = log.With(zap.Stringer("stream-id", segment.StreamID), zap.Uint64("position", segment.Position.Encode()))
	segmentData, failedDownloads, err := downloadSegment(ctx, log, peer, metabaseDB, segment)
	if err != nil {
		log.Error("download failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "download failed", len(segment.Pieces), failedDownloads)
		return
	}

	if err := reuploadSegment(ctx, log, peer, metabaseDB, segment, segmentData); err != nil {
		log.Error("upload failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "upload failed", len(segment.Pieces), failedDownloads)
		return
	}

	printOutput(segment.StreamID, segment.Position.Encode(), "successful", len(segment.Pieces), failedDownloads)
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

	putLimits, putPrivateKey, err := peer.Orders.Service.CreatePutRepairOrderLimits(ctx, segment, make([]*pb.AddressedOrderLimit, len(newNodes)),
		make(map[uint16]struct{}), newNodes)
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

	fec, err := eestream.NewFEC(redundancy.RequiredCount(), redundancy.TotalCount())
	if err != nil {
		return nil, failedDownloads, err
	}

	esScheme := eestream.NewUnsafeRSScheme(fec, redundancy.ErasureShareSize())
	pieceSize := segment.PieceSize()
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
