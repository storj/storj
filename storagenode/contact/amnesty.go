// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/trust"
)

const (
	defaultAmnestyBatchSize     = 50
	defaultAmnestyFlushInterval = 5 * time.Second
)

// AmnestyClient handles reporting bad pieces to satellites with batching and connection reuse.
type AmnestyClient struct {
	log    *zap.Logger
	dialer rpc.Dialer
	trust  trust.TrustedSatelliteSource

	// Batching configuration
	batchSize     int
	flushInterval time.Duration

	// Per-satellite batching state
	mu       sync.Mutex
	batches  map[storj.NodeID]*satelliteBatch
	shutdown bool
	wg       sync.WaitGroup
}

// satelliteBatch holds pending reports for a specific satellite
type satelliteBatch struct {
	satellite storj.NodeID
	pieces    []*pb.LostPiece
	timer     *time.Timer
}

// NewAmnestyClient creates a new amnesty client with batching enabled.
func NewAmnestyClient(log *zap.Logger, dialer rpc.Dialer, trust trust.TrustedSatelliteSource) *AmnestyClient {
	return &AmnestyClient{
		log:    log,
		dialer: dialer,
		trust:  trust,

		batchSize:     defaultAmnestyBatchSize,
		flushInterval: defaultAmnestyFlushInterval,

		batches: make(map[storj.NodeID]*satelliteBatch),
	}
}

// Close shuts down the client and flushes any pending reports.
func (ac *AmnestyClient) Close() error {
	ac.mu.Lock()
	ac.shutdown = true // ensure no new wg.Add calls can happen.

	batches := make([]*satelliteBatch, 0, len(ac.batches))
	for _, batch := range ac.batches {
		batches = append(batches, batch)

		// Stop any running timers as a best effort since we're going to send
		// the batch off anyway.
		if batch.timer != nil {
			batch.timer.Stop()
			batch.timer = nil
		}
	}
	ac.mu.Unlock()

	// Send remaining reports synchronously
	for _, batch := range batches {
		if len(batch.pieces) > 0 {
			ac.sendBatch(context.Background(), batch.satellite, batch.pieces)
		}
	}

	ac.wg.Wait()
	return nil
}

// ReportBadPiece adds a bad piece report to the batch for the given satellite.
// Reports are sent in batches to improve efficiency.
func (ac *AmnestyClient) ReportBadPiece(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) error {
	ac.log.Debug("adding bad piece to batch",
		zap.Stringer("satellite", satellite),
		zap.Stringer("piece_id", pieceID),
	)

	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.shutdown {
		return errs.New("amnesty client is shutting down")
	}

	batch, exists := ac.batches[satellite]
	if !exists {
		batch = &satelliteBatch{
			satellite: satellite,
			pieces:    make([]*pb.LostPiece, 0, ac.batchSize),
		}
		ac.batches[satellite] = batch
	}

	batch.pieces = append(batch.pieces, &pb.LostPiece{
		PieceId: pieceID,
		Reason:  pb.LostPieceReason_HASH_MISMATCH,
	})

	// If we've reached the batch size, send immediately
	if len(batch.pieces) >= ac.batchSize {
		ac.flushBatch(batch)
		return nil
	}

	// If this is the first piece in the batch, start the timer
	if len(batch.pieces) == 1 {
		batch.timer = time.AfterFunc(ac.flushInterval, func() {
			ac.mu.Lock()
			defer ac.mu.Unlock()

			ac.flushBatch(batch)
		})
	}

	return nil
}

// flushBatch sends the batch immediately (must be called with ac.mu held)
func (ac *AmnestyClient) flushBatch(batch *satelliteBatch) {
	if len(batch.pieces) == 0 {
		return
	}

	// Stop the timer if it's running
	if batch.timer != nil {
		batch.timer.Stop()
		batch.timer = nil
	}

	// If we're shutting down, don't start new goroutines: this races with the
	// wg.Wait in the Close call.
	if ac.shutdown {
		return
	}

	// Grab the pieces to send and clear the batch. We don't reuse the memory
	// because in a steady state, nothing will need amnesty, so it would just
	// be a memory leak after the initial burst.
	pieces := batch.pieces
	batch.pieces = nil

	ac.wg.Add(1)
	go func() {
		defer ac.wg.Done()
		ac.sendBatch(context.Background(), batch.satellite, pieces)
	}()
}

// sendBatch sends a batch of reports to the satellite
func (ac *AmnestyClient) sendBatch(ctx context.Context, satellite storj.NodeID, pieces []*pb.LostPiece) {
	if len(pieces) == 0 {
		return
	}

	logger := ac.log.With(
		zap.Stringer("satellite", satellite),
		zap.Int("piece_count", len(pieces)),
	)

	logger.Info("sending amnesty batch to satellite")

	nodeURL, err := ac.trust.GetNodeURL(ctx, satellite)
	if err != nil {
		logger.Error("failed to get satellite URL for amnesty report", zap.Error(err))
		return
	}

	conn, err := ac.dialer.DialNodeURL(ctx, nodeURL)
	if err != nil {
		logger.Error("failed to dial satellite for amnesty report", zap.Error(err))
		return
	}
	defer func() { _ = conn.Close() }()

	if _, err := pb.NewDRPCNodeClient(conn).AmnestyReport(ctx, &pb.AmnestyReportRequest{
		LostPieces: pieces,
	}); err != nil {
		logger.Error("failed to send amnesty report", zap.Error(err))
		return
	}

	logger.Debug("successfully sent amnesty batch")
}
