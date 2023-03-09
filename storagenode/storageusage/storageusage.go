// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// DB works with storage usage database.
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
	GetDailyTotal(ctx context.Context, from, to time.Time) ([]StampGroup, error)
	// Summary returns aggregated storage usage across all satellites.
	Summary(ctx context.Context, from, to time.Time) (float64, float64, error)
	// SatelliteSummary returns aggregated storage usage for a particular satellite.
	SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (float64, float64, error)
}

// Stamp is storage usage stamp for satellite from interval start till next interval.
type Stamp struct {
	SatelliteID storj.NodeID `json:"-"`
	// AtRestTotal is the bytes*hour disk space used at the IntervalEndTime.
	AtRestTotal float64 `json:"atRestTotal"`
	// AtRestTotalBytes is the AtRestTotal divided by the IntervalInHours.
	AtRestTotalBytes float64 `json:"atRestTotalBytes"`
	// IntervalInHours is hour difference between interval_end_time
	//  of this Stamp and that of the preceding Stamp
	IntervalInHours float64 `json:"intervalInHours"`
	// IntervalStart represents one tally day
	//  TODO: rename to timestamp to match DB
	IntervalStart time.Time `json:"intervalStart"`
	// IntervalEndTime represents the timestamp for the last tally run time
	//  (i.e. last interval_end_time) for the day
	IntervalEndTime time.Time `json:"-"`
}

// StampGroup is storage usage stamp for all satellites from interval start till next interval
// grouped by interval_start time.
type StampGroup struct {
	// AtRestTotal is the bytes*hour disk space used at the IntervalEndTime.
	AtRestTotal float64 `json:"atRestTotal"`
	// AtRestTotalBytes is the AtRestTotal divided by the IntervalInHours.
	AtRestTotalBytes float64 `json:"atRestTotalBytes"`
	// IntervalStart represents one tally day
	//  TODO: rename to timestamp to match DB
	IntervalStart time.Time `json:"intervalStart"`
}
