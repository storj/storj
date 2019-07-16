// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// DiskSpaceUsages is interface for working with disk space usage db
type DiskSpaceUsages interface {
	// Store stores disk space usage stamps to db
	Store(ctx context.Context, stamps []DiskSpaceUsage) error
	// GetDaily returns daily disk usage for particular satellite
	// for provided time range
	GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]DiskSpaceUsage, error)
	// GetDailyTotal returns daily disk usage summed across all known satellites
	// for provided time range
	GetDailyTotal(ctx context.Context, from, to time.Time) ([]DiskSpaceUsage, error)
}

// DiskSpaceInfo stores all info about storagenode disk space usage
type DiskSpaceInfo struct {
	Available int64 `json:"available"`
	Used      int64 `json:"used"`
}

// DiskSpaceUsage is space usage for satellite at some point in time
type DiskSpaceUsage struct {
	RollupID    int64
	SatelliteID storj.NodeID

	AtRestTotal float64

	Timestamp time.Time
}
