// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v49 = MultiDBState{
	Version: 49,
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
			SQL: `
				-- tables to hold payments and paystub data
				CREATE TABLE paystubs (
					period text NOT NULL,
					satellite_id bytea NOT NULL,
					created_at timestamp NOT NULL,
					codes text NOT NULL,
					usage_at_rest double precision NOT NULL,
					usage_get bigint NOT NULL,
					usage_put bigint NOT NULL,
					usage_get_repair bigint NOT NULL,
					usage_put_repair bigint NOT NULL,
					usage_get_audit bigint NOT NULL,
					comp_at_rest bigint NOT NULL,
					comp_get bigint NOT NULL,
					comp_put bigint NOT NULL,
					comp_get_repair bigint NOT NULL,
					comp_put_repair bigint NOT NULL,
					comp_get_audit bigint NOT NULL,
					surge_percent bigint NOT NULL,
					held bigint NOT NULL,
					owed bigint NOT NULL,
					disposed bigint NOT NULL,
					paid bigint NOT NULL,
					distributed bigint,
					PRIMARY KEY ( period, satellite_id )
				);
				CREATE TABLE payments (
					id bigserial NOT NULL,
					created_at timestamp NOT NULL,
					satellite_id bytea NOT NULL,
					period text,
					amount bigint NOT NULL,
					receipt text,
					notes text,
					PRIMARY KEY ( id )
				);`,
		},
		storagenodedb.PricingDBName: v47.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName: v47.DBStates[storagenodedb.APIKeysDBName]},
}
