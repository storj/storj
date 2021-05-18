// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v52 = MultiDBState{
	Version: 52,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v47.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v47.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v48.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v47.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v47.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v47.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v47.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v47.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v47.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v47.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v47.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName: &DBState{
			SQL: v51.DBStates[storagenodedb.HeldAmountDBName].SQL,
		},
		storagenodedb.PricingDBName: v47.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName: v47.DBStates[storagenodedb.APIKeysDBName],
		storagenodedb.PlannedDowntimeDBName: &DBState{
			SQL: `
				-- table to hold planned downtime data
				CREATE TABLE planned_downtime (
					id BLOB UNIQUE NOT NULL,
					start TIMESTAMP NOT NULL,
					end TIMESTAMP NOT NULL,
					scheduled_at TIMESTAMP NOT NULL
				);`,
		},
	},
}
