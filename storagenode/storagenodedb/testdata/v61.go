// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v61 = MultiDBState{
	Version: 61,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:  v55.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName: v55.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName: &DBState{
			SQL: `
				CREATE TABLE piece_space_used (
					total INTEGER NOT NULL DEFAULT 0,
					content_size INTEGER NOT NULL,
					satellite_id BLOB
				);

				CREATE UNIQUE INDEX idx_piece_space_used_satellite_id ON piece_space_used(satellite_id);
				INSERT INTO piece_space_used (content_size, total, satellite_id) VALUES (1337, 1337, X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000');
				INSERT INTO piece_space_used (content_size, total, satellite_id) VALUES (0, 0, X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3001');
			`,
		},
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
		storagenodedb.UsedSpacePerPrefixDBName:   v56.DBStates[storagenodedb.UsedSpacePerPrefixDBName],
	},
}
