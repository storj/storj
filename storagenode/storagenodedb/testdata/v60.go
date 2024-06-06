// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v60 = MultiDBState{
	Version: 60,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:    v55.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:   v55.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.PieceSpaceUsedDBName: v55.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:      v55.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: &DBState{
			SQL: `
				-- table to hold expiration data (and only expirations. no other pieceinfo)
				CREATE TABLE piece_expirations (
					satellite_id     BLOB      NOT NULL,
					piece_id         BLOB      NOT NULL,
					piece_expiration TIMESTAMP NOT NULL  -- date when it can be deleted
				);
				CREATE INDEX idx_piece_expirations_piece_expiration ON piece_expirations(piece_expiration);
			`,
		},
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
