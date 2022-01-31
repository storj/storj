// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"storj.io/storj/storagenode/storagenodedb"
)

var v43 = MultiDBState{
	Version: 43,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v42.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v41.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v41.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v41.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v41.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v41.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v41.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v41.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v41.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v41.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v41.DBStates[storagenodedb.NotificationsDBName],
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
		storagenodedb.PricingDBName: v41.DBStates[storagenodedb.PricingDBName],
	},
}
