// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// NodeOfflineTime represents a record in the nodes_offline_time table
type NodeOfflineTime struct {
	NodeID      storj.NodeID
	TrackedAt   time.Time
	TimeOffline time.Duration
}

// DB implements basic operations for downtime tracking service
//
// architecture: Database
type DB interface {
	// Add adds a record for a particular node ID with the amount of time it has been offline.
	Add(ctx context.Context, nodeID storj.NodeID, trackedTime time.Time, timeOffline time.Duration) error
	// GetOfflineTime gets the total amount of offline time for a node within a certain timeframe.
	// "total offline time" is defined as the sum of all offline time windows that begin inside the provided time window.
	// An offline time window that began before `begin` but that overlaps with the provided time window is not included.
	// An offline time window that begins within the provided time window, but that extends beyond `end` is included.
	GetOfflineTime(ctx context.Context, nodeID storj.NodeID, begin, end time.Time) (time.Duration, error)
}
