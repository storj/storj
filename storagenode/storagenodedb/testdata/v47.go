// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v47 = MultiDBState{
	Version: 47,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:  v46.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName: v46.DBStates[storagenodedb.StorageUsageDBName],
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
					audit_unknown_reputation_alpha REAL NOT NULL,
					audit_unknown_reputation_beta REAL NOT NULL,
					audit_unknown_reputation_score REAL NOT NULL,
					online_score REAL NOT NULL,
					audit_history BLOB,
					disqualified_at TIMESTAMP,
					updated_at TIMESTAMP NOT NULL,
					suspended_at TIMESTAMP,
					offline_suspended_at TIMESTAMP,
					offline_under_review_at TIMESTAMP,
					joined_at TIMESTAMP NOT NULL,
					PRIMARY KEY (satellite_id)
				);
				INSERT INTO reputation VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1,1.0,1.0,1.0,1,1,1.0,1.0,1.0,1.0,1.0,1.0,1.0,NULL,'2019-07-19 20:00:00+00:00','2019-08-23 20:00:00+00:00',NULL,NULL,NULL,'1970-01-01 00:00:00+00:00');
			`,
		},
		storagenodedb.PieceSpaceUsedDBName:  v46.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v46.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v46.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v46.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v46.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v46.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v46.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v46.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v46.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:         v46.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName:         v46.DBStates[storagenodedb.APIKeysDBName],
	},
}
