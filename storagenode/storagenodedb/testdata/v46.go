// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v46 = MultiDBState{
	Version: 46,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v43.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v43.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v45.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v43.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v43.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v43.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v43.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v43.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v43.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v43.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v43.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v43.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:         v43.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName: &DBState{
			SQL: `
				-- table to hold storagenode secret token
				CREATE TABLE secret (
					token bytea NOT NULL,
					created_at timestamp with time zone NOT NULL,
					PRIMARY KEY ( token )
				);`,
		},
	},
}
