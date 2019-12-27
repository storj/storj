// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/bloomfilter"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for retain errors.
	Error = errs.Class("retain")
)

// Config defines parameters for the retain service.
type Config struct {
	MaxTimeSkew time.Duration `help:"allows for small differences in the satellite and storagenode clocks" default:"72h0m0s"`
	Status      Status        `help:"allows configuration to enable, disable, or test retain requests from the satellite. Options: (disabled/enabled/debug)" default:"enabled"`
	Concurrency int           `help:"how many concurrent retain requests can be processed at the same time." default:"5"`
}

// Request contains all the info necessary to process a retain request.
type Request struct {
	SatelliteID   storj.NodeID
	CreatedBefore time.Time
	Filter        *bloomfilter.Filter
}

// Status is a type defining the enabled/disabled status of retain requests.
type Status uint32

const (
	// Disabled means we do not do anything with retain requests.
	Disabled Status = iota + 1
	// Enabled means we fully enable retain requests and delete data not defined by bloom filter.
	Enabled
	// Debug means we partially enable retain requests, and print out pieces we should delete, without actually deleting them.
	Debug
)

// Set implements pflag.Value.
func (v *Status) Set(s string) error {
	switch s {
	case "disabled":
		*v = Disabled
	case "enabled":
		*v = Enabled
	case "debug":
		*v = Debug
	default:
		return Error.New("invalid status %q", s)
	}
	return nil
}

// Type implements pflag.Value.
func (*Status) Type() string { return "storj.Status" }

// String implements pflag.Value.
func (v *Status) String() string {
	switch *v {
	case Disabled:
		return "disabled"
	case Enabled:
		return "enabled"
	case Debug:
		return "debug"
	default:
		return "invalid"
	}
}

// Service queues and processes retain requests from satellites.
//
// architecture: Worker
type Service struct {
	log    *zap.Logger
	config Config

	cond    sync.Cond
	queued  map[storj.NodeID]Request
	working map[storj.NodeID]struct{}
	group   errgroup.Group

	closedOnce sync.Once
	closed     chan struct{}
	started    bool

	store *pieces.Store
}

// NewService creates a new retain service.
func NewService(log *zap.Logger, store *pieces.Store, config Config) *Service {
	return &Service{
		log:    log,
		config: config,

		cond:    *sync.NewCond(&sync.Mutex{}),
		queued:  make(map[storj.NodeID]Request),
		working: make(map[storj.NodeID]struct{}),
		closed:  make(chan struct{}),

		store: store,
	}
}

// Queue adds a retain request to the queue.
// It discards a request for a satellite that already has a queued request.
// true is returned if the request is queued and false is returned if it is discarded
func (s *Service) Queue(req Request) bool {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	select {
	case <-s.closed:
		return false
	default:
	}

	s.queued[req.SatelliteID] = req
	s.cond.Broadcast()

	return true
}

// Run listens for queued retain requests and processes them as they come in.
func (s *Service) Run(ctx context.Context) error {
	// Hold the lock while we spawn the workers because a concurrent Close call
	// can race and wait for them. We later temporarily drop the lock while we
	// wait for the workers to exit.
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	// Ensure Run is only ever called once. If not, there's many subtle
	// bugs with concurrency.
	if s.started {
		return Error.New("service already started")
	}
	s.started = true

	// Ensure Run doesn't start after it's closed. Then we may leak some
	// workers.
	select {
	case <-s.closed:
		return Error.New("service Closed")
	default:
	}

	// Create a sub-context that we can cancel.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a goroutine that does most of the work of Close in the case
	// the context is canceled and broadcasts to anyone waiting on the condition
	// variable to check the state.
	s.group.Go(func() error {
		defer s.cond.Broadcast()
		defer cancel()

		select {
		case <-s.closed:
			return nil

		case <-ctx.Done():
			s.cond.L.Lock()
			s.closedOnce.Do(func() { close(s.closed) })
			s.cond.L.Unlock()

			return ctx.Err()
		}
	})

	for i := 0; i < s.config.Concurrency; i++ {
		s.group.Go(func() error {
			// Grab lock to check things.
			s.cond.L.Lock()
			defer s.cond.L.Unlock()

			for {
				// If we have closed, exit.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-s.closed:
					return nil
				default:
				}

				// Grab next item from queue.
				request, ok := s.next()
				if !ok {
					// Nothing in queue, go to sleep and wait for
					// things shutting down or next item.
					s.cond.Wait()
					continue
				}

				// Temporarily Unlock so others can work on the queue while
				// we're working on the retain request.
				s.cond.L.Unlock()

				// Signal to anyone waiting that the queue state changed.
				s.cond.Broadcast()

				// Run retaining process.
				err := s.retainPieces(ctx, request)
				if err != nil {
					s.log.Error("retain pieces failed", zap.Error(err))
				}

				// Mark the request as finished. Relock to maintain that
				// at the top of the for loop the lock is held.
				s.cond.L.Lock()
				s.finish(request)
				s.cond.Broadcast()
			}
		})
	}

	// Unlock while we wait for the workers to exit.
	s.cond.L.Unlock()
	err := s.group.Wait()
	s.cond.L.Lock()

	// Clear the queue after Wait has exited. We're sure no more entries
	// can be added after we acquire the mutex because wait spawned a
	// worker that ensures the closed channel is closed before it exits.
	s.queued = nil
	s.cond.Broadcast()

	return err
}

// next returns next item from queue, requires mutex to be held
func (s *Service) next() (Request, bool) {
	for id, request := range s.queued {
		// Check whether a worker is retaining this satellite,
		// if, yes, then try to get something else from the queue.
		if _, ok := s.working[request.SatelliteID]; ok {
			continue
		}
		delete(s.queued, id)
		// Mark this satellite as being worked on.
		s.working[request.SatelliteID] = struct{}{}
		return request, true
	}
	return Request{}, false
}

// finish marks the request as finished, requires mutex to be held
func (s *Service) finish(request Request) {
	delete(s.working, request.SatelliteID)
}

// Close causes any pending Run to exit and waits for any retain requests to
// clean up.
func (s *Service) Close() error {
	s.cond.L.Lock()
	s.closedOnce.Do(func() { close(s.closed) })
	s.cond.L.Unlock()

	s.cond.Broadcast()
	// ignoring error here, because the same error is already returned from Run.
	_ = s.group.Wait()
	return nil
}

// TestWaitUntilEmpty blocks until the queue and working is empty.
// When Run exits, it empties the queue.
func (s *Service) TestWaitUntilEmpty() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	for len(s.queued) > 0 || len(s.working) > 0 {
		s.cond.Wait()
	}
}

// Status returns the retain status.
func (s *Service) Status() Status {
	return s.config.Status
}

// ------------------------------------------------------------------------------------------------
// On the correctness of using access.ModTime() in place of the more precise access.CreationTime()
// in retainPieces():
// ------------------------------------------------------------------------------------------------
//
// Background: for pieces not stored with storage.FormatV0, the access.CreationTime() value can
// only be retrieved by opening the piece file, and reading and unmarshaling the piece header.
// This is far slower than access.ModTime(), which gets the file modification time from the file
// system and only needs to do a stat(2) on the piece file. If we can make Retain() work with
// ModTime, we should.
//
// Possibility of mismatch: We do not force or require piece file modification times to be equal to
// or close to the CreationTime specified by the uplink, but we do expect that piece files will be
// written to the filesystem _after_ the CreationTime. We make the assumption already that storage
// nodes and satellites and uplinks have system clocks that are very roughly in sync (that is, they
// are out of sync with each other by less than an hour of real time, or whatever is configured as
// MaxTimeSkew). So if an uplink is not lying about CreationTime and it uploads a piece that
// makes it to a storagenode's disk as quickly as possible, even in the worst-synchronized-clocks
// case we can assume that `ModTime > (CreationTime - MaxTimeSkew)`. We also allow for storage
// node operators doing file system manipulations after a piece has been written. If piece files
// are copied between volumes and their attributes are not preserved, it will be possible for their
// modification times to be changed to something later in time. This still preserves the inequality
// relationship mentioned above, `ModTime > (CreationTime - MaxTimeSkew)`. We only stipulate
// that storage node operators must not artificially change blob file modification times to be in
// the past.
//
// If there is a mismatch: in most cases, a mismatch between ModTime and CreationTime has no
// effect. In certain remaining cases, the only effect is that a piece file which _should_ be
// garbage collected survives until the next round of garbage collection. The only really
// problematic case is when there is a relatively new piece file which was created _after_ this
// node's Retain bloom filter started being built on the satellite, and is recorded in this
// storage node's blob store before the Retain operation has completed. Then, it might be possible
// for that new piece to be garbage collected incorrectly, because it does not show up in the
// bloom filter and the node incorrectly thinks that it was created before the bloom filter.
// But if the uplink is not lying about CreationTime and its clock drift versus the storage node
// is less than `MaxTimeSkew`, and the ModTime on a blob file is correctly set from the
// storage node system time, then it is still true that `ModTime > (CreationTime -
// MaxTimeSkew)`.
//
// The rule that storage node operators need to be aware of is only this: do not artificially set
// mtimes on blob files to be in the past. Let the filesystem manage mtimes. If blob files need to
// be moved or copied between locations, and this updates the mtime, that is ok. A secondary effect
// of this rule is that if the storage node's system clock needs to be changed forward by a
// nontrivial amount, mtimes on existing blobs should also be adjusted (by the same interval,
// ideally, but just running "touch" on all blobs is sufficient to avoid incorrect deletion of
// data).
func (s *Service) retainPieces(ctx context.Context, req Request) (err error) {
	// if retain status is disabled, return immediately
	if s.config.Status == Disabled {
		return nil
	}

	defer mon.Task()(&ctx, req.SatelliteID, req.CreatedBefore, req.Filter.Size())(&err)

	numDeleted := 0
	satelliteID := req.SatelliteID
	filter := req.Filter

	// subtract some time to leave room for clock difference between the satellite and storage node
	createdBefore := req.CreatedBefore.Add(-s.config.MaxTimeSkew)

	s.log.Debug("Prepared to run a Retain request.",
		zap.Time("Created Before", createdBefore),
		zap.Int64("Filter Size", filter.Size()),
		zap.Stringer("Satellite ID", satelliteID))

	err = s.store.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
		// We call Gosched() when done because the GC process is expected to be long and we want to keep it at low priority,
		// so other goroutines can continue serving requests.
		defer runtime.Gosched()
		// See the comment above the retainPieces() function for a discussion on the correctness
		// of using ModTime in place of the more precise CreationTime.
		mTime, err := access.ModTime(ctx)
		if err != nil {
			s.log.Warn("failed to determine mtime of blob", zap.Error(err))
			// but continue iterating.
			return nil
		}
		if !mTime.Before(createdBefore) {
			return nil
		}
		pieceID := access.PieceID()
		if !filter.Contains(pieceID) {
			s.log.Debug("About to delete piece id",
				zap.Stringer("Satellite ID", satelliteID),
				zap.Stringer("Piece ID", pieceID),
				zap.String("Status", s.config.Status.String()))

			// if retain status is enabled, delete pieceid
			if s.config.Status == Enabled {
				if err = s.store.Trash(ctx, satelliteID, pieceID); err != nil {
					s.log.Warn("failed to delete piece",
						zap.Stringer("Satellite ID", satelliteID),
						zap.Stringer("Piece ID", pieceID),
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
	s.log.Debug("Deleted pieces during retain", zap.Int("num deleted", numDeleted), zap.String("Retain Status", s.config.Status.String()))

	return nil
}
