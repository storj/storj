// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// DB works with reputation database
type DB interface {
	// Store inserts or updates reputation stats into the DB
	Store(ctx context.Context, stats Stats) error
	// Get retrieves stats for specific satellite
	Get(ctx context.Context, satelliteID storj.NodeID) (*Stats, error)
}

// Stats consist of reputation metrics
type Stats struct {
	SatelliteID storj.NodeID

	Uptime Metric
	Audit  Metric

	UpdatedAt time.Time
}

// Metric encapsulates storagenode reputation metrics
type Metric struct {
	TotalCount   int64
	SuccessCount int64

	Alpha float64
	Beta  float64
	Score float64
}
