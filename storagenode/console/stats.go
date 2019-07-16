// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// Stats is interface for working with node stats db
type Stats interface {
	// Create inserts new stats into the db
	Create(ctx context.Context, stats NodeStats) (*NodeStats, error)
	// Update updates stored stats
	Update(ctx context.Context, stats NodeStats) error
	// Get retrieves stats for specific satellite
	Get(ctx context.Context, satelliteID storj.NodeID) (*NodeStats, error)
}

// NodeStats encapsulates storagenode stats retrieved from the satellite
type NodeStats struct {
	SatelliteID storj.NodeID

	UptimeCheck ReputationStats
	AuditCheck  ReputationStats

	UpdatedAt time.Time
}

// ReputationStats encapsulates storagenode reputation metrics
type ReputationStats struct {
	TotalCount   int64
	SuccessCount int64

	ReputationAlpha float64
	ReputationBeta  float64
	ReputationScore float64
}
