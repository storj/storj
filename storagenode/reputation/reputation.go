// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// DB works with reputation database
//
// architecture: Database
type DB interface {
	// Store inserts or updates reputation stats into the DB
	Store(ctx context.Context, stats Stats) error
	// Get retrieves stats for specific satellite
	Get(ctx context.Context, satelliteID storj.NodeID) (*Stats, error)
	// All retrieves all stats from DB
	All(ctx context.Context) ([]Stats, error)
}

// Stats consist of reputation metrics
type Stats struct {
	SatelliteID storj.NodeID

	Uptime Metric
	Audit  Metric

	Disqualified *time.Time

	UpdatedAt time.Time
}

// Metric encapsulates storagenode reputation metrics
type Metric struct {
	TotalCount   int64 `json:"totalCount"`
	SuccessCount int64 `json:"successCount"`

	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	Score float64 `json:"score"`
}
