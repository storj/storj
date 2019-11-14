// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v26 = MultiDBState{
	Version: 26,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:    v25.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:   v25.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:     v25.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName: v25.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:      v25.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: &DBState{
			SQL: `
				-- table to hold expiration data (and only expirations. no other pieceinfo)
				CREATE TABLE piece_expirations (
					satellite_id       BLOB      NOT NULL,
					piece_id           BLOB      NOT NULL,
					piece_expiration   TIMESTAMP NOT NULL, -- date when it can be deleted
					deletion_failed_at TIMESTAMP,
					trash              INTEGER NOT NULL DEFAULT 0,
					PRIMARY KEY ( satellite_id, piece_id )
				);
				CREATE INDEX idx_piece_expirations_piece_expiration ON piece_expirations(piece_expiration);
				CREATE INDEX idx_piece_expirations_deletion_failed_at ON piece_expirations(deletion_failed_at);
				CREATE INDEX idx_piece_expirations_trashed ON piece_expirations(satellite_id, trash) WHERE trash = 1;
			`,
		},
		storagenodedb.OrdersDBName:         v25.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:      v25.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:     v25.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName: v25.DBStates[storagenodedb.DeprecatedInfoDBName],
	},
}
