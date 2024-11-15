// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/satstore"
)

// ShouldTrashFunc is an alias for the callback that is used to determine if a piece should be trashed.
type ShouldTrashFunc = func(ctx context.Context, pieceID storj.PieceID, created time.Time) bool

// BloomFilterManager manages a directory that holds the most recent bloom filter for satellites and
// allows getting a callback to query them.
type BloomFilterManager struct {
	ss *satstore.SatelliteStore

	mu sync.Mutex
	m  map[storj.NodeID]*atomic.Pointer[bloomFilterState]
}

// Status implements the piecestore.QueueRetain interface.
func (bfm *BloomFilterManager) Status() Status {
	return Store
}

type bloomFilterState struct {
	filter  *bloomfilter.Filter
	created time.Time
}

// NewBloomFilterManager constructs a BloomFilterManager with the given directory. This function
// does not do the standard pattern where an error means the return result is invalid. Indeed, the
// result is always valid, and the errors are purely informational.
func NewBloomFilterManager(dir string) (*BloomFilterManager, error) {
	bfm := &BloomFilterManager{
		ss: satstore.NewSatelliteStore(dir, "bloomfilter"),

		m: make(map[storj.NodeID]*atomic.Pointer[bloomFilterState]),
	}

	return bfm, bfm.ss.Range(func(satellite storj.NodeID, data []byte) error {
		req := new(pb.RetainRequest)
		if err := pb.Unmarshal(data, req); err != nil {
			return err
		}
		if err := verifyHash(req); err != nil {
			return err
		}
		filter, err := bloomfilter.NewFromBytes(req.Filter)
		if err != nil {
			return err
		}
		bfm.getStatePtrLocked(satellite).Store(&bloomFilterState{
			filter:  filter,
			created: req.CreationDate,
		})
		return nil
	})
}

func (bfm *BloomFilterManager) getStatePtrLocked(satellite storj.NodeID) *atomic.Pointer[bloomFilterState] {
	statePtr, ok := bfm.m[satellite]
	if !ok {
		statePtr = new(atomic.Pointer[bloomFilterState])
		bfm.m[satellite] = statePtr
	}
	return statePtr
}

// Queue stores the RetainRequest for the satellite and sets the current bloom filter to be based on it.
// It will not update the bloom filter unless the created time increases.
func (bfm *BloomFilterManager) Queue(ctx context.Context, satellite storj.NodeID, req *pb.RetainRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	bfm.mu.Lock()
	defer bfm.mu.Unlock()

	if err := verifyHash(req); err != nil {
		return errs.Wrap(err)
	}

	statePtr := bfm.getStatePtrLocked(satellite)

	if existing := statePtr.Load(); existing != nil && existing.created.After(req.CreationDate) {
		return nil
	}

	filter, err := bloomfilter.NewFromBytes(req.Filter)
	if err != nil {
		return errs.Wrap(err)
	}

	// always use the new filter even if we have problems persisting it to disk.
	statePtr.Store(&bloomFilterState{
		filter:  filter,
		created: req.CreationDate,
	})

	data, err := pb.Marshal(req)
	if err != nil {
		return errs.Wrap(err)
	}

	return bfm.ss.Set(ctx, satellite, data)
}

// GetBloomFilter returns a ShouldTrashFunc for the given satellite that always queries whatever the latest
// bloom filter is for the given satellite.
func (bfm *BloomFilterManager) GetBloomFilter(satellite storj.NodeID) ShouldTrashFunc {
	bfm.mu.Lock()
	defer bfm.mu.Unlock()

	statePtr := bfm.getStatePtrLocked(satellite)

	return func(ctx context.Context, pieceID storj.PieceID, created time.Time) bool {
		state := statePtr.Load()
		return state != nil && created.Before(state.created) && !state.filter.Contains(pieceID)
	}
}
