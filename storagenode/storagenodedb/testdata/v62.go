// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v62 = MultiDBState{
	Version: 62,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:          v55.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:         v55.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName:       v61.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:            v55.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName:      v60.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:               v55.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:            v57.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:           v55.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:       v55.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:        v55.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:           v55.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:              v55.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName:              v55.DBStates[storagenodedb.APIKeysDBName],
		storagenodedb.GCFilewalkerProgressDBName: v55.DBStates[storagenodedb.GCFilewalkerProgressDBName],
		storagenodedb.UsedSpacePerPrefixDBName: &DBState{
			SQL: `
				CREATE TABLE used_space_per_prefix (
				    satellite_id BLOB NOT NULL,
				    piece_prefix TEXT NOT NULL,
				    total_bytes INTEGER NOT NULL,
				    total_content_size INTEGER NOT NULL DEFAULT 0,
				    piece_counts INTEGER NOT NULL DEFAULT 0,
				    last_updated TIMESTAMP NOT NULL,
				    PRIMARY KEY (satellite_id, piece_prefix)
				);`,
		},
	},
}
