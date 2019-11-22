// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"fmt"
	"sync"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"
)

// memoryLiveAccounting represents an accounting.Cache-implementing
// instance using plain memory (no coordination with other servers). It can be
// used to coordinate tracking of how much space a project has used.
//
// This should probably only be used at small scale or for testing areas where
// the accounting cache does not matter significantly. For production, an
// implementation that allows multiple servers to participate together would
// be preferable.
type memoryLiveAccounting struct {
	log *zap.Logger

	spaceMapLock sync.RWMutex
	spaceDeltas  map[uuid.UUID]int64
}

func newMemoryLiveAccounting(log *zap.Logger) (*memoryLiveAccounting, error) {
	pmac := &memoryLiveAccounting{log: log}
	pmac.spaceDeltas = make(map[uuid.UUID]int64)
	return pmac, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (mac *memoryLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error) {
	defer mon.Task()(&ctx, projectID)(&err)
	mac.spaceMapLock.Lock()
	defer mac.spaceMapLock.Unlock()
	curVal, ok := mac.spaceDeltas[projectID]
	fmt.Println("cam entire cache", mac.spaceDeltas)
	fmt.Println("cam getting project total for", projectID, curVal)
	if !ok {
		return 0, nil
	}
	return curVal, nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added spaceUsed
func (mac *memoryLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, spaceUsed int64) (err error) {
	defer mon.Task()(&ctx, projectID, spaceUsed)(&err)
	mac.spaceMapLock.Lock()
	defer mac.spaceMapLock.Unlock()
	curVal := mac.spaceDeltas[projectID]
	newTotal := curVal + spaceUsed
	mac.spaceDeltas[projectID] = newTotal
	fmt.Println("cam added project total for", projectID, newTotal)
	return nil
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (mac *memoryLiveAccounting) ResetTotals(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	fmt.Println("cam resetting totals")
	mac.log.Debug("Resetting real-time accounting data")
	mac.spaceMapLock.Lock()
	mac.spaceDeltas = make(map[uuid.UUID]int64)
	mac.spaceMapLock.Unlock()
	return nil
}

// GetAllProjectTotals iterates through the live accounting DB and returns a map of project IDs and totals
func (mac *memoryLiveAccounting) GetAllProjectTotals(ctx context.Context) (map[uuid.UUID]int64, error) {
	return mac.spaceDeltas, nil
}

// Close matches the accounting.LiveAccounting interface.
func (mac *memoryLiveAccounting) Close() error { return nil }
