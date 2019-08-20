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
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for retain errors.
	Error = errs.Class("retain")
)

// Config defines parameters for the retain service.
type Config struct {
	RetainTimeBuffer    time.Duration `help:"allows for small differences in the satellite and storagenode clocks" default:"1h0m0s"`
	RetainStatus        Status        `help:"allows configuration to enable, disable, or test retain requests from the satellite. Options: (disabled/enabled/debug)" default:"disabled"`
	MaxConcurrentRetain int           `help:"how many concurrent retain requests can be processed at the same time." default:"5"`
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
		return Error.New("invalid RetainStatus %q", s)
	}
	return nil
}

// Type implements pflag.Value.
func (*Status) Type() string { return "storj.RetainStatus" }

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
type Service struct {
	log    *zap.Logger
	config Config

	mu     sync.Mutex
	queued map[storj.NodeID]Request

	reqChan      chan Request
	semaphore    chan struct{}
	emptyTrigger chan struct{}

	store *pieces.Store
}

// NewService creates a new retain service.
func NewService(log *zap.Logger, store *pieces.Store, config Config) *Service {
	return &Service{
		log:          log,
		config:       config,
		queued:       make(map[storj.NodeID]Request),
		reqChan:      make(chan Request),
		semaphore:    make(chan struct{}, config.MaxConcurrentRetain),
		emptyTrigger: make(chan struct{}),
		store:        store,
	}
}

// Queue adds a retain request to the queue.
// It discards a request for a satellite that already has a queued request.
// true is returned if the request is queued and false is returned if it is discarded
func (s *Service) Queue(req Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// subtract some time to leave room for clock difference between the satellite and storage node
	req.CreatedBefore = req.CreatedBefore.Add(-s.config.RetainTimeBuffer)

	// only queue retain request if we do not already have one for this satellite
	if _, ok := s.queued[req.SatelliteID]; !ok {
		s.queued[req.SatelliteID] = req
		go func() { s.reqChan <- req }()

		return true
	}

	return false
}

// Run listens for queued retain requests and processes them as they come in.
func (s *Service) Run(ctx context.Context) error {
	for {
		// exit if context has been canceled. Otherwise, block until an item can be added to the semaphore
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s.semaphore <- struct{}{}:
		}

		// get the next request
		var req Request
		select {
		case req = <-s.reqChan:
		case <-ctx.Done():
			return ctx.Err()
		}

		go func(ctx context.Context, req Request) {
			err := s.retainPieces(ctx, req)
			if err != nil {
				s.log.Error("retain error", zap.Error(err))
			}
			s.mu.Lock()
			delete(s.queued, req.SatelliteID)
			queueLength := len(s.queued)
			s.mu.Unlock()

			if queueLength == 0 {
				s.emptyTrigger <- struct{}{}
			}

			// remove item from semaphore and free up process for another retain job
			<-s.semaphore
		}(ctx, req)
	}
}

// Wait blocks until the context is canceled or until the queue is empty.
func (s *Service) Wait(ctx context.Context) {
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

// Status returns the retain status.
func (s *Service) Status() Status {
	return s.config.RetainStatus
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
// RetainTimeBuffer). So if an uplink is not lying about CreationTime and it uploads a piece that
// makes it to a storagenode's disk as quickly as possible, even in the worst-synchronized-clocks
// case we can assume that `ModTime > (CreationTime - RetainTimeBuffer)`. We also allow for storage
// node operators doing file system manipulations after a piece has been written. If piece files
// are copied between volumes and their attributes are not preserved, it will be possible for their
// modification times to be changed to something later in time. This still preserves the inequality
// relationship mentioned above, `ModTime > (CreationTime - RetainTimeBuffer)`. We only stipulate
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
// is less than `RetainTimeBuffer`, and the ModTime on a blob file is correctly set from the
// storage node system time, then it is still true that `ModTime > (CreationTime -
// RetainTimeBuffer)`.
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
	if s.config.RetainStatus == Disabled {
		return nil
	}

	defer mon.Task()(&ctx, req.SatelliteID, req.CreatedBefore, req.Filter.Size())(&err)

	numDeleted := 0
	satelliteID := req.SatelliteID
	filter := req.Filter
	createdBefore := req.CreatedBefore

	s.log.Debug("Prepared to run a Retain request.",
		zap.Time("createdBefore", createdBefore),
		zap.Int64("filterSize", filter.Size()),
		zap.String("satellite", satelliteID.String()))

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
				zap.String("satellite", satelliteID.String()),
				zap.String("pieceID", pieceID.String()),
				zap.String("status", s.config.RetainStatus.String()))

			// if retain status is enabled, delete pieceid
			if s.config.RetainStatus == Enabled {
				if err = s.store.Delete(ctx, satelliteID, pieceID); err != nil {
					s.log.Warn("failed to delete piece",
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
	s.log.Debug("Deleted pieces during retain", zap.Int("num deleted", numDeleted), zap.String("retain status", s.config.RetainStatus.String()))

	return nil
}
