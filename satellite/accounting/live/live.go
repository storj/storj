// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"strings"
	"sync"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Config contains configurable values for the live accounting service.
type Config struct {
	StorageBackend string `help:"what to use for storing real-time accounting data" default:"plainmemory"`
}

// Service represents the external interface to the live accounting
// functionality.
type Service interface {
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
	AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error
	ResetTotals()
}

// New creates a new live.Service instance of the type specified in
// the provided config.
func New(log *zap.Logger, config Config) (Service, error) {
	parts := strings.SplitN(config.StorageBackend, ":", 2)
	var backendType string
	if len(parts) == 0 || parts[0] == "" {
		backendType = "plainmemory"
	} else {
		backendType = parts[0]
	}
	if backendType == "plainmemory" {
		return newPlainMemoryLiveAccounting(log)
	}
	return nil, errs.New("unrecognized live accounting backend specifier %q", backendType)
}

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
	spaceDeltas  map[uuid.UUID]spaceUsedAccounting
}

type spaceUsedAccounting struct {
	inlineSpace int64
	remoteSpace int64
}

func newPlainMemoryLiveAccounting(log *zap.Logger) (*plainMemoryLiveAccounting, error) {
	pmac := &plainMemoryLiveAccounting{log: log}
	pmac.ResetTotals()
	return pmac, nil
}

// GetProjectStorageUsage gets inline and remote storage totals for a given
// project, back to the time of the last accounting tally.
func (pmac *plainMemoryLiveAccounting) GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (inlineTotal, remoteTotal int64, err error) {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal := pmac.spaceDeltas[projectID]
	return curVal.inlineSpace, curVal.remoteSpace, nil
}

// AddProjectStorageUsage lets the live accounting know that the given
// project has just added inlineSpaceUsed bytes of inline space usage
// and remoteSpaceUsed bytes of remote space usage.
func (pmac *plainMemoryLiveAccounting) AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error {
	pmac.spaceMapLock.Lock()
	defer pmac.spaceMapLock.Unlock()
	curVal := pmac.spaceDeltas[projectID]
	curVal.inlineSpace += inlineSpaceUsed
	curVal.remoteSpace += remoteSpaceUsed
	pmac.spaceDeltas[projectID] = curVal
	return nil
}

// ResetTotals reset all space-used totals for all projects back to zero. This
// would normally be done in concert with calculating new tally counts in the
// accountingDB.
func (pmac *plainMemoryLiveAccounting) ResetTotals() {
	pmac.log.Info("Resetting real-time accounting data")
	pmac.spaceMapLock.Lock()
	pmac.spaceDeltas = make(map[uuid.UUID]spaceUsedAccounting)
	pmac.spaceMapLock.Unlock()
}
