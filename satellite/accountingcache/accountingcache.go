// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accountingcache

import (
	"context"
	"strings"
	"sync"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Config contains configurable values for the accountingcache service.
type Config struct {
	StorageBackend string `help:"What to use for storing real-time accounting data"`
}

// Service represents the external interface to the accountingcache
// functionality.
type Service interface {
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
	AddSpaceUsed(projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error
	ResetTotals()
}

// New creates a new accountingcache.Service instance of the type specified in
// the provided config.
func New(log *zap.Logger, config Config) (Service, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		backendType = "plainmemory"
	} else {
		backendType = parts[0]
	}
	switch backendType {
	case "plainmemory":
		return newPlainMemoryAccountingCache(log)
	}
	return nil, errs.New("Unrecognized accountingcache backend specifier %q", backendType)
}

// plainMemoryAccountingCache represents an accountingcache.Service-implementing
// instance using plain memory (no coordination with other servers). It can be
// used to coordinate tracking of how much space and bandwidth an uplink has
// used.
//
// This should probably only be used at small scale or for testing areas where
// the accounting cache does not matter significantly. For production, an
// implementation that allows multiple servers to participate together would
// be preferable.
type plainMemoryAccountingCache struct {
	log *zap.Logger

	spaceMapLock     sync.RWMutex
	spaceDeltas      map[uuid.UUID]spaceUsedAccounting
	bandwidthMapLock sync.RWMutex
	bandwidthDeltas  map[uuid.UUID]int64
}

type spaceUsedAccounting struct {
	inlineSpace int64
	remoteSpace int64
}

func newPlainMemoryAccountingCache(log *zap.Logger) (*plainMemoryAccountingCache, error) {
	pmac := &plainMemoryAccountingCache{log: log}
	pmac.ResetTotals()
	return pmac, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (pmac *plainMemoryAccountingCache) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (inlineTotal, remoteTotal int64, err error) {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal := pmac.spaceDeltas[projectID]
	return curVal.inlineSpace, curVal.remoteSpace, nil
}

// AddSpaceUsed lets the accountingcache know that the given project has just
// added spaceUsed bytes of usage.
func (pmac *plainMemoryAccountingCache) AddSpaceUsed(projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal := pmac.spaceDeltas[projectID]
	curVal.inlineSpace += inlineSpaceUsed
	curVal.remoteSpace += remoteSpaceUsed
	pmac.spaceDeltas[projectID] = curVal
	return nil
}

// ResetTotals reset all space-used and bandwidth-used totals for all projects
// back to zero. This would normally be done in concert with calculating new
// tally counts in the accountingDB.
func (pmac *plainMemoryAccountingCache) ResetTotals() {
	pmac.log.Info("Resetting real-time accounting data")
	pmac.spaceMapLock.Lock()
	pmac.spaceDeltas = make(map[uuid.UUID]spaceUsedAccounting)
	pmac.spaceMapLock.Unlock()
	pmac.bandwidthMapLock.Lock()
	pmac.bandwidthDeltas = make(map[uuid.UUID]int64)
	pmac.bandwidthMapLock.Unlock()
}
