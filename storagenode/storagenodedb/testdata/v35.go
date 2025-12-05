// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v35 = MultiDBState{
	Version: 35,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v28.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v28.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v34.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v31.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v28.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v28.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v28.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v28.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v28.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v28.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v28.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v33.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName: &DBState{
			SQL: `
				-- tables to hold pricing model data
				CREATE TABLE pricing (
					satellite_id BLOB NOT NULL,
					egress_bandwidth_price bigint NOT NULL,
					repair_bandwidth_price bigint NOT NULL,
					audit_bandwidth_price bigint NOT NULL,
					disk_space_price bigint NOT NULL,
					PRIMARY KEY ( satellite_id )
				);`,
		},
	},
}
