// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/common/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type downtimeTrackingDB struct {
	db *satelliteDB
}

// Add adds a record for a particular node ID with the amount of time it has been offline.
func (db *downtimeTrackingDB) Add(ctx context.Context, nodeID storj.NodeID, trackedTime time.Time, timeOffline time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Create_NodesOfflineTime(ctx,
		dbx.NodesOfflineTime_NodeId(nodeID.Bytes()),
		dbx.NodesOfflineTime_TrackedAt(trackedTime),
		dbx.NodesOfflineTime_Seconds(int(timeOffline.Seconds())),
	)

	return Error.Wrap(err)
}

// GetOfflineTime gets the total amount of offline time for a node within a certain timeframe.
// "total offline time" is defined as the sum of all offline time windows that begin inside the provided time window.
// An offline time window that began before `begin` but that overlaps with the provided time window is not included.
// An offline time window that begins within the provided time window, but that extends beyond `end` is included.
func (db *downtimeTrackingDB) GetOfflineTime(ctx context.Context, nodeID storj.NodeID, begin, end time.Time) (time.Duration, error) {
	offlineEntries, err := db.db.All_NodesOfflineTime_By_NodeId_And_TrackedAt_Greater_And_TrackedAt_LessOrEqual(ctx,
		dbx.NodesOfflineTime_NodeId(nodeID.Bytes()),
		dbx.NodesOfflineTime_TrackedAt(begin),
		dbx.NodesOfflineTime_TrackedAt(end),
	)
	if err != nil {
		return time.Duration(0), Error.Wrap(err)
	}

	totalSeconds := 0
	for _, entry := range offlineEntries {
		totalSeconds += entry.Seconds
	}
	duration := time.Duration(totalSeconds) * time.Second
	return duration, nil
}
