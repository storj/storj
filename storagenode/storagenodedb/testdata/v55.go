// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v55 = MultiDBState{
	Version: 55,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v54.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v54.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName:  v54.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v54.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v54.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v54.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v54.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v54.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v54.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v54.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v54.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:         v54.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName:         v54.DBStates[storagenodedb.APIKeysDBName],
		storagenodedb.GCFilewalkerProgressDBName: &DBState{
			SQL: `
				CREATE TABLE progress (
					satellite_id BLOB NOT NULL,
					bloomfilter_created_before TIMESTAMP NOT NULL,
					last_checked_prefix TEXT NOT NULL,
					PRIMARY KEY (satellite_id)
				);
			`,
		},
	},
}
