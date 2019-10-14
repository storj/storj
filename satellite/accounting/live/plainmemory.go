// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"sync"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"
)

// plainMemoryLiveAccounting represents an live.Service-implementing
// instance using plain memory (no coordination with other servers). It can be
// used to coordinate tracking of how much space a project has used.
//
// This should probably only be used at small scale or for testing areas where
// the accounting cache does not matter significantly. For production, an
// implementation that allows multiple servers to participate together would
// be preferable.
type plainMemoryLiveAccounting struct {
	log *zap.Logger

	spaceMapLock sync.RWMutex
	spaceDeltas  map[uuid.UUID]int64
}

func newPlainMemoryLiveAccounting(log *zap.Logger) (*plainMemoryLiveAccounting, error) {
	pmac := &plainMemoryLiveAccounting{log: log}
	pmac.spaceMapLock.Lock()
	pmac.spaceDeltas = make(map[uuid.UUID]int64, 0)
	pmac.spaceMapLock.Unlock()
	return pmac, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (pmac *plainMemoryLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal, ok := pmac.spaceDeltas[projectID]
	if !ok {
		return 0, nil
	}
	return curVal, nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added inlineSpaceUsed bytes of inline space usage
// and remoteSpaceUsed bytes of remote space usage.
func (pmac *plainMemoryLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal := pmac.spaceDeltas[projectID]
	newTotal := curVal + inlineSpaceUsed + remoteSpaceUsed
	pmac.spaceDeltas[projectID] = newTotal
	return nil
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (pmac *plainMemoryLiveAccounting) ResetTotals(ctx context.Context) error {
	pmac.log.Info("Resetting real-time accounting data")
	pmac.spaceMapLock.Lock()
	pmac.spaceDeltas = make(map[uuid.UUID]int64)
	pmac.spaceMapLock.Unlock()
	return nil
}

// Close matches the accounting.LiveAccounting interface
func (pmac *plainMemoryLiveAccounting) Close() error { return nil }
