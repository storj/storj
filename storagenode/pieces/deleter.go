// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
)

// DeleteRequest contains information to delete piece.
type DeleteRequest struct {
	SatelliteID storj.NodeID
	PieceID     storj.PieceID
	QueueTime   time.Time
}

// Deleter is a worker that processes requests to delete groups of pieceIDs.
// Deletes are processed "best-effort" asynchronously, and any errors are
// logged.
type Deleter struct {
	mu         sync.Mutex
	ch         chan DeleteRequest
	numWorkers int
	eg         *errgroup.Group
	log        *zap.Logger
	stop       func()
	store      *Store
	closed     bool

	// The test variables are only used when testing.
	testMode     bool
	testToDelete int
	testDone     chan struct{}
}

// NewDeleter creates a new Deleter.
func NewDeleter(log *zap.Logger, store *Store, numWorkers int, queueSize int) *Deleter {
	if numWorkers == 0 {
		numWorkers = 1
	}
	if queueSize == 0 {
		// Default queueSize is chosen as a large number that uses a manageable
		// amount of memory.
		queueSize = 10000
	}
	return &Deleter{
		ch:         make(chan DeleteRequest, queueSize),
		numWorkers: numWorkers,
		log:        log,
		store:      store,
	}
}

// Run starts the delete workers.
func (d *Deleter) Run(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return errs.New("already closed")
	}

	if d.stop != nil {
		return errs.New("already started")
	}

	ctx, d.stop = context.WithCancel(ctx)
	d.eg = &errgroup.Group{}

	for i := 0; i < d.numWorkers; i++ {
		d.eg.Go(func() error {
			return d.work(ctx)
		})
	}

	return nil
}

// Enqueue adds the pieceIDs to the delete queue. If the queue is full deletes
// are not processed and will be left for garbage collection. Enqueue returns
// true if all pieceIDs were successfully placed on the queue, false if some
// pieceIDs were dropped.
func (d *Deleter) Enqueue(ctx context.Context, satelliteID storj.NodeID, pieceIDs []storj.PieceID) (unhandled int) {
	if len(pieceIDs) == 0 {
		return 0
	}

	// If we are in testMode add the number of pieceIDs waiting to be processed.
	if d.testMode {
		d.checkDone(len(pieceIDs))
	}

	for i, pieceID := range pieceIDs {
		select {
		case d.ch <- DeleteRequest{satelliteID, pieceID, time.Now()}:
		default:
			unhandled := len(pieceIDs) - i
			mon.Counter("piecedeleter-queue-full").Inc(1)
			if d.testMode {
				d.checkDone(-unhandled)
			}
			return unhandled
		}
	}

	return 0
}

func (d *Deleter) checkDone(delta int) {
	d.mu.Lock()
	d.testToDelete += delta
	if d.testToDelete < 0 {
		d.testToDelete = 0
	} else if d.testToDelete == 0 {
		if d.testDone != nil {
			close(d.testDone)
			d.testDone = nil
		}
	} else if d.testToDelete > 0 {
		if d.testDone == nil {
			d.testDone = make(chan struct{})
		}
	}
	d.mu.Unlock()
}

func (d *Deleter) work(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-d.ch:
			mon.IntVal("piecedeleter-queue-time").Observe(int64(time.Since(r.QueueTime)))
			mon.IntVal("piecedeleter-queue-size").Observe(int64(len(d.ch)))
			err := d.store.Delete(ctx, r.SatelliteID, r.PieceID)
			if err != nil {
				// If a piece cannot be deleted, we just log the error.
				d.log.Error("delete failed",
					zap.Stringer("Satellite ID", r.SatelliteID),
					zap.Stringer("Piece ID", r.PieceID),
					zap.Error(err),
				)
			} else {
				d.log.Info("deleted",
					zap.Stringer("Satellite ID", r.SatelliteID),
					zap.Stringer("Piece ID", r.PieceID),
				)
			}

			// If we are in test mode, check if we are done processing deletes
			if d.testMode {
				d.checkDone(-1)
			}
		}
	}
}

// Close stops all the workers and waits for them to finish.
func (d *Deleter) Close() error {
	d.mu.Lock()
	d.closed = true
	stop := d.stop
	eg := d.eg
	d.mu.Unlock()

	if stop != nil {
		stop()
	}
	if eg != nil {
		return eg.Wait()
	}
	return nil
}

// Wait blocks until the queue is empty and each enqueued delete has been
// successfully processed.
func (d *Deleter) Wait(ctx context.Context) {
	d.mu.Lock()
	testDone := d.testDone
	d.mu.Unlock()
	if testDone != nil {
		select {
		case <-ctx.Done():
		case <-testDone:
		}
	}
}

// SetupTest puts the deleter in test mode. This should only be called in tests.
func (d *Deleter) SetupTest() {
	d.testMode = true
}
