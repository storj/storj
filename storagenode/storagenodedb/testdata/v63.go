// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v63 = MultiDBState{
	Version: 63,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:          v62.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:         v62.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName:       v62.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:            v62.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName:      v62.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:               v62.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:            v62.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:           v62.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:       v62.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.HeldAmountDBName:           v62.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:              v62.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName:              v62.DBStates[storagenodedb.APIKeysDBName],
		storagenodedb.GCFilewalkerProgressDBName: v62.DBStates[storagenodedb.GCFilewalkerProgressDBName],
		storagenodedb.UsedSpacePerPrefixDBName:   v62.DBStates[storagenodedb.UsedSpacePerPrefixDBName],
		storagenodedb.NotificationsDBName: &DBState{
			SQL: `
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
				ALTER TABLE notifications ADD COLUMN link TEXT NOT NULL DEFAULT '';
				ALTER TABLE notifications ADD COLUMN link_label TEXT NOT NULL DEFAULT '';
			`,
		},
	},
}
