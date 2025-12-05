// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// GCFilewalkerProgressDB is used to store intermediate progress to resume garbage
// collection after restarting the node.
type GCFilewalkerProgressDB interface {
	// Store stores the GC filewalker progress.
	Store(ctx context.Context, progress GCFilewalkerProgress) error
	// Get returns the GC filewalker progress for the satellite.
	Get(ctx context.Context, satelliteID storj.NodeID) (GCFilewalkerProgress, error)
	// Reset resets the GC filewalker progress for the satellite.
	Reset(ctx context.Context, satelliteID storj.NodeID) error
}

// UsedSpacePerPrefixDB is used to store the intermediate progress to resume
// the used space calculation after restarting the node.
type UsedSpacePerPrefixDB interface {
	// Store stores the used space per prefix.
	Store(ctx context.Context, usedSpace PrefixUsedSpace) error
	// StoreBatch stores the used space per prefix in batch.
	StoreBatch(ctx context.Context, usedSpaces []PrefixUsedSpace) error
	// Get returns the used space per prefix for the satellite, for prefixes that were updated after lastUpdated.
	Get(ctx context.Context, satelliteID storj.NodeID, lastUpdated *time.Time) ([]PrefixUsedSpace, error)
	// GetSatelliteUsedSpace returns the total used space for the satellite.
	GetSatelliteUsedSpace(ctx context.Context, satelliteID storj.NodeID) (piecesTotal, piecesContentSize, piecesCount int64, err error)
	// Delete deletes the used space per prefix for the satellite.
	Delete(ctx context.Context, satelliteID storj.NodeID) error
}

// GCFilewalkerProgress keeps track of the GC filewalker progress.
type GCFilewalkerProgress struct {
	Prefix                   string
	SatelliteID              storj.NodeID
	BloomfilterCreatedBefore time.Time
}

// PrefixUsedSpace contains the used space information of a prefix.
type PrefixUsedSpace struct {
	Prefix           string
	SatelliteID      storj.NodeID
	TotalBytes       int64
	TotalContentSize int64
	PieceCounts      int64
	LastUpdated      time.Time
}
