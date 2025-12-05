// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v36 = MultiDBState{
	Version: 36,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:  v28.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName: v28.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName: &DBState{
			SQL: `
				-- tables to store nodestats cache
				CREATE TABLE reputation (
					satellite_id BLOB NOT NULL,
					uptime_success_count INTEGER NOT NULL,
					uptime_total_count INTEGER NOT NULL,
					uptime_reputation_alpha REAL NOT NULL,
					uptime_reputation_beta REAL NOT NULL,
					uptime_reputation_score REAL NOT NULL,
					audit_success_count INTEGER NOT NULL,
					audit_total_count INTEGER NOT NULL,
					audit_reputation_alpha REAL NOT NULL,
					audit_reputation_beta REAL NOT NULL,
					audit_reputation_score REAL NOT NULL,
					disqualified TIMESTAMP,
					suspended TIMESTAMP,
					updated_at TIMESTAMP NOT NULL,
					joined_at TIMESTAMP,
					PRIMARY KEY (satellite_id)
				);
				INSERT INTO reputation VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1,1.0,1.0,1.0,1,1,1.0,1.0,1.0,'2019-07-19 20:00:00+00:00',NULL,'2019-08-23 20:00:00+00:00',NULL);
			`,
		},
		storagenodedb.PieceSpaceUsedDBName:  v31.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v28.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v28.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v28.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v28.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v28.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v28.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v28.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v33.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:         v35.DBStates[storagenodedb.PricingDBName],
	},
}
