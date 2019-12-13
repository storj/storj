// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"storj.io/storj/storagenode/storagenodedb"
)

var v27 = MultiDBState{
	Version: 27,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v26.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v26.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v26.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v26.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v26.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v26.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v26.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v26.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v26.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v26.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName: &DBState{
			SQL: `
				-- table to hold notifications data
				CREATE TABLE notifications (
					id         BLOB NOT NULL,
					sender_id  BLOB NOT NULL,
					type       INTEGER NOT NULL,
					title      TEXT NOT NULL,
					message    TEXT NOT NULL,
					read_at    TIMESTAMP,
					created_at TIMESTAMP NOT NULL,
					PRIMARY KEY (id)
				);
			`,
		},
	},
}
