// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// DB works with storage usage database
type DB interface {
	// Store stores storage usage stamps to db replacing conflicting entries
	Store(ctx context.Context, stamps []Stamp) error
	// GetDaily returns daily storage usage stamps for particular satellite
	// for provided time range
	GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]Stamp, error)
	// GetDailyTotal returns daily storage usage stamps summed across all known satellites
	// for provided time range
	GetDailyTotal(ctx context.Context, from, to time.Time) ([]Stamp, error)
}

// Stamp is storage usage stamp for satellite at some point in time
type Stamp struct {
	SatelliteID storj.NodeID `json:"-"`

	AtRestTotal float64 `json:"atRestTotal"`

	Timestamp time.Time `json:"timestamp"`
}
