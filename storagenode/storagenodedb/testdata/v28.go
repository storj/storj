// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"storj.io/storj/storagenode/storagenodedb"
)

var v28 = MultiDBState{
	Version: 28,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v27.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v27.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v27.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v27.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v27.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v27.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v27.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v27.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v27.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v27.DBStates[storagenodedb.DeprecatedInfoDBName],
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
