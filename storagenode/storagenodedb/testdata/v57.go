// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v57 = MultiDBState{
	Version: 57,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v55.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v55.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName:  v55.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v55.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v55.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v55.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName: {
			SQL: `
			CREATE TABLE bandwidth_usage (
    				interval_start TIMESTAMP NOT NULL,
    				satellite_id BLOB NOT NULL,
    				put_total BIGINT DEFAULT 0,
    				get_total BIGINT DEFAULT 0,
    				get_audit_total BIGINT DEFAULT 0,
    				get_repair_total BIGINT DEFAULT 0,
    				put_repair_total BIGINT DEFAULT 0,
    				delete_total BIGINT DEFAULT 0,
    				PRIMARY KEY (interval_start, satellite_id)
            );

			INSERT INTO bandwidth_usage VALUES ('2019-04-01 00:00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',2,4,6,8,10,12);
			INSERT INTO bandwidth_usage VALUES ('2019-07-12 00:00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,2,3,4,5,6);
			INSERT INTO bandwidth_usage VALUES ('2019-04-01 00:00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',2,4,6,8,10,12);
			INSERT INTO bandwidth_usage VALUES ('2019-07-12 00:00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',1,2,3,4,5,6);
		`,
		},
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
