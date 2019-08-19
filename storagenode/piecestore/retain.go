// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

// RetainService queues and processes retain requests from satellites
type RetainService struct {
	log          *zap.Logger
	retainStatus RetainStatus

	mu     sync.Mutex
	queued map[storj.NodeID]RetainRequest

	reqChan      chan RetainRequest
	sem          chan struct{}
	emptyTrigger chan struct{}

	store *pieces.Store
}

// RetainRequest contains all the info necessary to process a retain request
type RetainRequest struct {
	SatelliteID   storj.NodeID
	CreatedBefore time.Time
	Filter        *bloomfilter.Filter
}

// NewRetainService creates a new retain service
func NewRetainService(log *zap.Logger, retainStatus RetainStatus, concurrentRetain int, store *pieces.Store) *RetainService {
	return &RetainService{
		log:          log,
		retainStatus: retainStatus,
		queued:       make(map[storj.NodeID]RetainRequest),
		reqChan:      make(chan RetainRequest),
		sem:          make(chan struct{}, concurrentRetain),
		emptyTrigger: make(chan struct{}),
		store:        store,
	}
}

// QueueRetain adds a retain request to the queue
func (s *RetainService) QueueRetain(req RetainRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// only queue retain request if we do not already have one for this satellite
	if _, ok := s.queued[req.SatelliteID]; !ok {
		s.queued[req.SatelliteID] = req
		go func() { s.reqChan <- req }()
	}
}

// Run listens for queued retain requests and processes them as they come in
func (s *RetainService) Run(ctx context.Context) error {
	for {
		// exit if context has been canceled. Otherwise, block until an item can be added to the semaphore
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s.sem <- struct{}{}:
		}

		// get the next request
		var req RetainRequest
		select {
		case req = <-s.reqChan:
		case <-ctx.Done():
			return ctx.Err()
		}

		// TODO make it possible to sync with this goroutine
		go func(ctx context.Context, req RetainRequest) {
			err := s.retainPieces(ctx, req)
			if err != nil {
				s.log.Error("retain error", zap.Error(err))
			}
			s.mu.Lock()
			delete(s.queued, req.SatelliteID)
			s.mu.Unlock()

			if len(s.queued) == 0 {
				s.emptyTrigger <- struct{}{}
			}

			// remove item from semaphore and free up process for another retain job
			<-s.sem
		}(ctx, req)
	}
}

// Wait blocks until the context is canceled or until the queue is empty
func (s *RetainService) Wait(ctx context.Context) {
	s.mu.Lock()
	queueLength := len(s.queued)
	s.mu.Unlock()
	if queueLength == 0 {
		return
	}
	select {
	case <-s.emptyTrigger:
	case <-ctx.Done():
	}
}

func (s *RetainService) retainPieces(ctx context.Context, req RetainRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	// if retain status is disabled, return immediately
	if s.retainStatus == RetainDisabled {
		return nil
	}

	numDeleted := 0
	satelliteID := req.SatelliteID
	filter := req.Filter
	createdBefore := req.CreatedBefore

	s.log.Info("Prepared to run a Retain request.",
		zap.Time("createdBefore", createdBefore),
		zap.Int64("filterSize", filter.Size()),
		zap.String("satellite", satelliteID.String()))

	err = s.store.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
		// We call Gosched() when done because the GC process is expected to be long and we want to keep it at low priority,
		// so other goroutines can continue serving requests.
		defer runtime.Gosched()
		// See the comment above the Retain() function for a discussion on the correctness
		// of using ModTime in place of the more precise CreationTime.
		mTime, err := access.ModTime(ctx)
		if err != nil {
			s.log.Error("failed to determine mtime of blob", zap.Error(err))
			// but continue iterating.
			return nil
		}
		if !mTime.Before(createdBefore) {
			return nil
		}
		pieceID := access.PieceID()
		if !filter.Contains(pieceID) {
			s.log.Debug("About to delete piece id",
				zap.String("satellite", satelliteID.String()),
				zap.String("pieceID", pieceID.String()),
				zap.String("retainStatus", s.retainStatus.String()))

			// if retain status is enabled, delete pieceid
			if s.retainStatus == RetainEnabled {
				if err = s.store.Delete(ctx, satelliteID, pieceID); err != nil {
					s.log.Error("failed to delete piece",
						zap.String("satellite", satelliteID.String()),
						zap.String("pieceID", pieceID.String()),
						zap.Error(err))
					return nil
				}
			}
			numDeleted++
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}
	mon.IntVal("garbage_collection_pieces_deleted").Observe(int64(numDeleted))
	s.log.Sugar().Debugf("Deleted %d pieces during retain. RetainStatus: %s", numDeleted, s.retainStatus.String())

	return nil
}
