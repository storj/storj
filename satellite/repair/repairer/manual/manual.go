// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package manual

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/shared/modular"
	"storj.io/uplink/private/eestream"
)

// Repairer provides manual repair functionality for satellite segments.
type Repairer struct {
	log             *zap.Logger
	metabase        *metabase.DB
	overlayDB       overlay.DB
	overlay         *overlay.Service
	orders          *orders.Service
	ecRepairer      *repairer.ECRepairer
	segmentRepairer *repairer.SegmentRepairer
	queue           queue.Consumer
	trigger         *modular.StopTrigger
}

// NewRepairer creates a new manual repairer instance.
func NewRepairer(
	log *zap.Logger,
	metabase *metabase.DB,
	overlayDB overlay.DB,
	overlay *overlay.Service,
	orders *orders.Service,
	ecRepairer *repairer.ECRepairer,
	segmentRepairer *repairer.SegmentRepairer,
	queue queue.Consumer,

	trigger *modular.StopTrigger,
) *Repairer {
	return &Repairer{
		log:             log,
		metabase:        metabase,
		overlayDB:       overlayDB,
		overlay:         overlay,
		orders:          orders,
		ecRepairer:      ecRepairer,
		segmentRepairer: segmentRepairer,
		queue:           queue,
		trigger:         trigger,
	}
}

// Run executes the manual repair process for all configured segments.
func (m *Repairer) Run(ctx context.Context) (err error) {
	defer m.trigger.Cancel()
	for {
		if ctx.Err() != nil {
			return nil
		}

		segments, err := m.queue.Select(ctx, 10, nil, nil)
		if errors.Is(err, io.EOF) && len(segments) == 0 {
			return nil
		}
		if err != nil {
			return errs.Wrap(err)
		}
		for _, segment := range segments {
			m.Handle(ctx, segment)
		}
	}
}

// RepairSegment will repair selected segment no matter if it's healthy or not.
//
// Logic for this method is:
// * download whole segment into memory, use all available pieces
// * reupload segment into new nodes
// * replace segment.Pieces field with just new nodes.
func (m *Repairer) RepairSegment(ctx context.Context, segment metabase.SegmentForRepair) bool {
	log := m.log.With(zap.Stringer("stream-id", segment.StreamID), zap.Uint64("position", segment.Position.Encode()))
	segmentData, failedDownloads, err := m.downloadSegment(ctx, segment)
	if err != nil {
		log.Error("download failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "download failed", len(segment.Pieces), failedDownloads)
		return false
	}

	if err := m.reuploadSegment(ctx, segment, segmentData); err != nil {
		log.Error("upload failed", zap.Error(err))

		printOutput(segment.StreamID, segment.Position.Encode(), "upload failed", len(segment.Pieces), failedDownloads)
		return false
	}

	printOutput(segment.StreamID, segment.Position.Encode(), "successful", len(segment.Pieces), failedDownloads)
	return true
}

func (m *Repairer) reuploadSegment(ctx context.Context, segment metabase.SegmentForRepair, segmentData []byte) error {
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

	newNodes, err := m.overlay.FindStorageNodesForUpload(ctx, request)
	if err != nil {
		return err
	}

	if len(newNodes) < redundancy.RepairThreshold() {
		return errs.New("not enough new nodes were found for repair: min %v got %v", redundancy.RepairThreshold(), len(newNodes))
	}

	putLimits, putPrivateKey, err := m.orders.CreatePutRepairOrderLimits(ctx, segment, segment.Redundancy, make([]*pb.AddressedOrderLimit, len(newNodes)),
		make(map[uint16]struct{}), newNodes)
	if err != nil {
		return errs.New("could not create PUT_REPAIR order limits: %w", err)
	}

	timeout := 5 * time.Minute
	successfulNeeded := redundancy.OptimalThreshold()
	successful, _, err := m.ecRepairer.Repair(ctx, m.log, putLimits, putPrivateKey, redundancy, bytes.NewReader(segmentData),
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
	return m.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
		StreamID: segment.StreamID,
		Position: segment.Position,

		OldPieces:     segment.Pieces,
		NewRedundancy: segment.Redundancy,
		NewPieces:     repairedPieces,

		NewRepairedAt: time.Now(),
	})
}

func (m *Repairer) downloadSegment(ctx context.Context, segment metabase.SegmentForRepair) ([]byte, int, error) {
	// AdminFetchPieces downloads all pieces for specified segment and returns readers, readers data is kept on disk or inmemory
	pieceInfos, err := m.segmentRepairer.AdminFetchPieces(ctx, m.log, &segment, "")
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

		log := m.log.With(zap.Int("piece num", pieceNum))

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

	m.log.Info("download summary",
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

// Handle processes a single injured segment for repair.
func (m *Repairer) Handle(ctx context.Context, injured queue.InjuredSegment) {
	var success bool
	defer func() {
		err := m.queue.Release(ctx, injured, success)
		if err != nil {
			m.log.Error("Couldn't release segment", zap.Stringer("stream-id", injured.StreamID), zap.Uint64("position", injured.Position.Encode()))
		}
	}()

	segment, err := m.metabase.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
		StreamID: injured.StreamID,
		Position: injured.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			printOutput(segment.StreamID, segment.Position.Encode(), "segment not found in metabase db", 0, 0)
			success = true
		} else {
			m.log.Error("unknown error when getting segment metadata",
				zap.Stringer("stream-id", segment.StreamID),
				zap.Uint64("position", segment.Position.Encode()),
				zap.Error(err))
			printOutput(segment.StreamID, segment.Position.Encode(), "internal", 0, 0)
		}
		return
	}
	success = m.RepairSegment(ctx, segment)
}

// printOutput prints result to standard output in a way to be able to combine
// single results into single csv file.
func printOutput(streamID uuid.UUID, position uint64, result string, numberOfPieces, failedDownloads int) {
	fmt.Printf("%s,%d,%s,%d,%d\n", streamID, position, result, numberOfPieces, failedDownloads)
}

// SegmentWithPosition represents a segment identifier with stream ID and position.
type SegmentWithPosition struct {
	StreamID uuid.UUID
	Position uint64
}
