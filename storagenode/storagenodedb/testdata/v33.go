// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v33 = MultiDBState{
	Version: 33,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v28.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v28.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v28.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v31.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v28.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v28.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v28.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v28.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v28.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v28.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v28.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName: &DBState{
			SQL: `
				-- tables to hold heldamount and payments data
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
	},
}
