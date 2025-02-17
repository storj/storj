// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/blobstore/filestore"
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
	Concurrency int           `help:"how many concurrent retain requests can be processed at the same time." default:"1"`
	CachePath   string        `help:"path to the cache directory for retain requests." default:"$CONFDIR/retain"`
}

// Request contains all the info necessary to process a retain request.
type Request struct {
	Filename      string
	SatelliteID   storj.NodeID
	CreatedBefore time.Time
	Filter        *bloomfilter.Filter
}

// GetFilename returns the filename used to store the request in the cache directory.
func (req *Request) GetFilename() string {
	if req.Filename != "" {
		return req.Filename
	}

	return fmt.Sprintf("%s-%s.pb",
		filestore.PathEncoding.EncodeToString(req.SatelliteID.Bytes()),
		strconv.FormatInt(req.CreatedBefore.UnixNano(), 10),
	)
}

// Queue manages the retain requests queue.
type Queue interface {
	// Add adds a request to the queue.
	Add(satelliteID storj.NodeID, request *pb.RetainRequest) (bool, error)
	// Remove removes a request from the queue.
	// Returns true if there was a request to remove.
	Remove(request Request) bool
	// Next returns the next request from the queue.
	Next() (Request, bool)
	// Len returns the number of requests in the queue.
	Len() int
	// DeleteCache removes the request from the queue and deletes the cache file.
	DeleteCache(request Request) error
	// MarkInProgress marks the request as in progress.
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
	// Store means the retain messages will be saved, but not processed.
	Store
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
	case "store":
		*v = Store
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
	queue   Queue
	working map[storj.NodeID]struct{}
	group   errgroup.Group

	closedOnce sync.Once
	closed     chan struct{}
	started    bool

	store *pieces.Store
}

// NewService creates a new retain service.
func NewService(log *zap.Logger, store *pieces.Store, config Config) *Service {
	log = log.With(zap.String("cachePath", config.CachePath))
	cache, err := NewRequestStore(config.CachePath)
	if err != nil {
		log.Warn("encountered error(s) while loading cache", zap.Error(err))
	}

	return &Service{
		log:    log,
		config: config,

		cond:    *sync.NewCond(&sync.Mutex{}),
		queue:   &cache,
		working: make(map[storj.NodeID]struct{}),
		closed:  make(chan struct{}),

		store: store,
	}
}

const (
	closedErrMsg = "Retain job not queued (queue is closed)"
)

// Queue adds a retain request to the queue.
// true is returned if the request is added to the queue, false if queue is closed.
func (s *Service) Queue(ctx context.Context, satelliteID storj.NodeID, req *pb.RetainRequest) error {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	select {
	case <-s.closed:

		s.log.Info(closedErrMsg, zap.Stringer("Satellite ID", satelliteID))
		return errs.New(closedErrMsg)
	default:
	}

	ok, err := s.queue.Add(satelliteID, req)
	if err != nil {
		s.log.Warn("encountered an error while adding request to queue", zap.Error(err), zap.Bool("Queued", ok), zap.Stringer("Satellite ID", satelliteID))
		return err
	}
	if ok {
		s.log.Info("Retain job queued", zap.Stringer("Satellite ID", satelliteID))
	} else {
		s.log.Info(closedErrMsg, zap.Stringer("Satellite ID", satelliteID))
	}

	s.cond.Broadcast()
	return nil
}

// Run listens for queued retain requests and processes them as they come in.
func (s *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

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
	concurrency := s.config.Concurrency
	if s.config.Status == Store {
		// we don't run the real loop, as it immediately deletes the BFs, what we need to store
		s.group.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-s.closed:
					return nil
				}
			}
		})

		// disable the real loop
		concurrency = 0

	}
	for i := 0; i < concurrency; i++ {
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
				successful := true
				err := s.retainPieces(ctx, request)
				if err != nil {
					s.log.Error("retain pieces failed", zap.Error(err))
					successful = false
				}

				// Mark the request as finished. Relock to maintain that
				// at the top of the for loop the lock is held.
				s.cond.L.Lock()
				s.finish(request, successful)
				s.cond.Broadcast()
			}
		})
	}

	// Unlock while we wait for the workers to exit.
	s.cond.L.Unlock()
	err = s.group.Wait()
	s.cond.L.Lock()

	// Clear the queue after Wait has exited. We're sure no more entries
	// can be added after we acquire the mutex because wait spawned a
	// worker that ensures the closed channel is closed before it exits.
	s.queue = nil
	s.cond.Broadcast()

	return err
}

// next returns next item from queue, requires mutex to be held.
func (s *Service) next() (Request, bool) {
	for {
		request, ok := s.queue.Next()
		if !ok {
			return Request{}, false
		}
		// Check whether a worker is retaining this satellite,
		// if, yes, then try to get something else from the queue.
		if _, ok := s.working[request.SatelliteID]; ok {
			continue
		}
		// Mark this satellite as being worked on.
		s.working[request.SatelliteID] = struct{}{}
		s.queue.Remove(request)
		return request, true
	}
}

// finish marks the request as finished and removes the cache, requires mutex to be held.
func (s *Service) finish(request Request, successful bool) {
	delete(s.working, request.SatelliteID)
	if successful {
		err := s.queue.DeleteCache(request)
		if err != nil {
			s.log.Warn("encountered an error while removing request from queue", zap.Error(err), zap.Stringer("Satellite ID", request.SatelliteID))
		}
	}
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

	for s.queue.Len() > 0 || len(s.working) > 0 {
		s.cond.Wait()
	}
}

// Status returns the retain status.
func (s *Service) Status() Status {
	return s.config.Status
}

func (s *Service) retainPieces(ctx context.Context, req Request) (err error) {
	// if retain status is disabled, return immediately
	if s.config.Status == Disabled || s.config.Status == Store {
		return nil
	}

	defer mon.Task()(&ctx, req.SatelliteID, req.CreatedBefore)(&err)

	satelliteID := req.SatelliteID
	filter := req.Filter

	// subtract some time to leave room for clock difference between the satellite and storage node
	createdBefore := req.CreatedBefore.Add(-s.config.MaxTimeSkew)
	startedAt := time.Now().UTC()
	numTrashed := 0
	filterHashCount, _ := req.Filter.Parameters()
	mon.IntVal("garbage_collection_created_before").Observe(createdBefore.Unix())
	mon.IntVal("garbage_collection_filter_hash_count").Observe(int64(filterHashCount))
	mon.IntVal("garbage_collection_filter_size").Observe(filter.Size())
	mon.IntVal("garbage_collection_started").Observe(startedAt.Unix())

	s.log.Info("Prepared to run a Retain request.",
		zap.Time("Created Before", createdBefore),
		zap.Int64("Filter Size", filter.Size()),
		zap.Stringer("Satellite ID", satelliteID))

	piecesToDeleteCount := 0
	piecesCount, piecesSkipped, err := s.store.WalkSatellitePiecesToTrash(ctx, satelliteID, createdBefore, filter, func(pieceID storj.PieceID) error {
		s.log.Debug("About to move piece to trash",
			zap.String("BF", req.Filename),
			zap.Stringer("Satellite ID", satelliteID),
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Status", &s.config.Status))

		piecesToDeleteCount++
		// if retain status is enabled, trash the piece
		if s.config.Status == Enabled {
			if err := s.store.Trash(ctx, satelliteID, pieceID, startedAt); err != nil {
				s.log.Warn("failed to trash piece",
					zap.Stringer("Satellite ID", satelliteID),
					zap.Stringer("Piece ID", pieceID),
					zap.Error(err))
				return nil
			}
		}

		numTrashed++

		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	mon.IntVal("garbage_collection_pieces_count").Observe(piecesCount)
	mon.IntVal("garbage_collection_pieces_skipped").Observe(piecesSkipped)
	mon.IntVal("garbage_collection_pieces_to_delete_count").Observe(int64(piecesToDeleteCount))
	mon.IntVal("garbage_collection_pieces_deleted").Observe(int64(numTrashed))
	duration := time.Now().UTC().Sub(startedAt)
	mon.DurationVal("garbage_collection_loop_duration").Observe(duration)
	s.log.Info("Moved pieces to trash during retain",
		zap.Int("Deleted pieces", numTrashed),
		zap.Int("Failed to delete", piecesToDeleteCount-numTrashed),
		zap.Int64("Pieces failed to read", piecesSkipped),
		zap.Int64("Pieces count", piecesCount),
		zap.Stringer("Satellite ID", satelliteID),
		zap.Duration("Duration", duration),
		zap.String("Retain Status", s.config.Status.String()),
	)

	return nil
}

// TestingHowManyQueued peeks at the number of bloom filters queued.
func (s *Service) TestingHowManyQueued() int {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	return s.queue.Len()
}
