// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// DB works with storage usage database
//
// architecture: Database
type DB interface {
	// Store stores storage usage stamps to db replacing conflicting entries
	Store(ctx context.Context, stamps []Stamp) error
	// GetDaily returns daily storage usage stamps for particular satellite
	// for provided time range
	GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]Stamp, error)
	// GetDailyTotal returns daily storage usage stamps summed across all known satellites
	// for provided time range
	GetDailyTotal(ctx context.Context, from, to time.Time) ([]Stamp, error)
	// Summary returns aggregated storage usage across all satellites.
	Summary(ctx context.Context, from, to time.Time) (float64, error)
	// SatelliteSummary returns aggregated storage usage for a particular satellite.
	SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (float64, error)
}

// Stamp is storage usage stamp for satellite from interval start till next interval.
type Stamp struct {
	SatelliteID   storj.NodeID `json:"-"`
	AtRestTotal   float64      `json:"atRestTotal"`
	IntervalStart time.Time    `json:"intervalStart"`
}
