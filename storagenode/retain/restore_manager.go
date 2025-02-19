// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/satstore"
)

// RestoreTimeManager keeps track of the latest timestamp that a restore was called per satellite.
type RestoreTimeManager struct {
	ss *satstore.SatelliteStore

	mu sync.Mutex
}

// NewRestoreTimeManager constructs a restoreManager using the given directory.
func NewRestoreTimeManager(dir string) *RestoreTimeManager {
	return &RestoreTimeManager{
		ss: satstore.NewSatelliteStore(dir, "restore"),
	}
}

// TestingSetRestoreTime sets the restore timestamp for the given satellite allowing it to go
// backwards.
func (r *RestoreTimeManager) TestingSetRestoreTime(ctx context.Context, satellite storj.NodeID, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.setLocked(ctx, satellite, now)
}

// SetRestoreTime sets the restore timestamp for the given satellite.
func (r *RestoreTimeManager) SetRestoreTime(ctx context.Context, satellite storj.NodeID, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	r.mu.Lock()
	defer r.mu.Unlock()

	// try to ensure that we only allow the restore timestamp to increase.
	if ts, ok := r.getLocked(ctx, satellite); ok && ts.After(now) {
		return nil
	}

	return r.setLocked(ctx, satellite, now)
}

func (r *RestoreTimeManager) setLocked(ctx context.Context, satellite storj.NodeID, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(now.Unix()))

	return r.ss.Set(ctx, satellite, buf[:])
}

// GetRestoreTime returns the latest restore timestamp for the given satellite. If there is no value,
// the now value is returned and attempted to be stored.
func (r *RestoreTimeManager) GetRestoreTime(ctx context.Context, satellite storj.NodeID, now time.Time) (_ time.Time) {
	defer mon.Task()(&ctx)(nil)

	r.mu.Lock()
	defer r.mu.Unlock()

	if ts, ok := r.getLocked(ctx, satellite); ok {
		return ts
	}

	// if we failed to get, try to set. no big deal if it fails: the next get will try again.
	_ = r.setLocked(ctx, satellite, now)
	return now
}

func (r *RestoreTimeManager) getLocked(ctx context.Context, satellite storj.NodeID) (time.Time, bool) {
	data, err := r.ss.Get(ctx, satellite)
	if err == nil && len(data) == 8 {
		return time.Unix(int64(binary.BigEndian.Uint64(data)), 0), true
	}
	return time.Time{}, false
}
