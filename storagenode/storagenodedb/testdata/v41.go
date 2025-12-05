// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"storj.io/storj/storagenode/storagenodedb"
)

var v41 = MultiDBState{
	Version: 41,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v40.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v40.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v40.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v40.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v40.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v40.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v40.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v40.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName: &DBState{
			SQL: `
				CREATE TABLE satellites (
					node_id BLOB NOT NULL,
					added_at TIMESTAMP NOT NULL,
					status INTEGER NOT NULL,
					PRIMARY KEY (node_id)
				);
				CREATE TABLE satellite_exit_progress (
					satellite_id BLOB NOT NULL,
					initiated_at TIMESTAMP,
					finished_at TIMESTAMP,
					starting_disk_usage INTEGER NOT NULL,
					bytes_deleted INTEGER NOT NULL,
					completion_receipt BLOB,
					FOREIGN KEY (satellite_id) REFERENCES satellites (node_id)
				);
				INSERT INTO satellites VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000','2019-09-10 20:00:00+00:00', 0);
				INSERT INTO satellite_exit_progress VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000','2019-09-10 20:00:00+00:00', null, 100, 0, null);
			`,
		},
		storagenodedb.DeprecatedInfoDBName: v40.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:  v40.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:     v40.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:        v40.DBStates[storagenodedb.PricingDBName],
	},
}
