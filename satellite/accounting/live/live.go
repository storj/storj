// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"context"
	"strings"

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
//
// architecture: Service
type Service interface {
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
	AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error
	ResetTotals(ctx context.Context) error
}

type spaceUsedAccounting struct {
	InlineSpace int64
	RemoteSpace int64
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
	switch backendType {
	case "plainmemory":
		return newPlainMemoryLiveAccounting(log)
	case "redis":
		return newRedisLiveAccounting(log, config.StorageBackend)
	default:
		return nil, errs.New("unrecognized live accounting backend specifier %q", backendType)
	}
}
