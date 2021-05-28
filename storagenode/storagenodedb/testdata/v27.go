// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v27 = MultiDBState{
	Version: 27,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     v26.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName:    v26.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v26.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v26.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v26.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v26.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName: &DBState{
			SQL: `
				-- table for storing all unsent orders
				CREATE TABLE unsent_order (
					satellite_id  BLOB NOT NULL,
					serial_number BLOB NOT NULL,
					order_limit_serialized BLOB      NOT NULL,
					order_serialized       BLOB      NOT NULL,
					order_limit_expiration TIMESTAMP NOT NULL,
					uplink_cert_id INTEGER NOT NULL,
					FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
				);
				CREATE UNIQUE INDEX idx_orders ON unsent_order(satellite_id, serial_number);
				-- table for storing all sent orders
				CREATE TABLE order_archive_ (
					satellite_id  BLOB NOT NULL,
					serial_number BLOB NOT NULL,
					order_limit_serialized BLOB NOT NULL,
					order_serialized       BLOB NOT NULL,
					uplink_cert_id INTEGER NOT NULL,
					status      INTEGER   NOT NULL,
					archived_at TIMESTAMP NOT NULL,
					FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
				);
				CREATE INDEX idx_order_archived_at ON order_archive_(archived_at);
				INSERT INTO unsent_order VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',X'1eddef484b4c03f01332279032796972',X'0a101eddef484b4c03f0133227903279697212202b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf410001a201968996e7ef170a402fdfd88b6753df792c063c07c555905ffac9cd3cbd1c00022200ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac30002a20d00cf14f3c68b56321ace04902dec0484eb6f9098b22b31c6b3f82db249f191630643802420c08dfeb88e50510a8c1a5b9034a0c08dfeb88e50510a8c1a5b9035246304402204df59dc6f5d1bb7217105efbc9b3604d19189af37a81efbf16258e5d7db5549e02203bb4ead16e6e7f10f658558c22b59c3339911841e8dbaae6e2dea821f7326894',X'0a101eddef484b4c03f0133227903279697210321a47304502206d4c106ddec88140414bac5979c95bdea7de2e0ecc5be766e08f7d5ea36641a7022100e932ff858f15885ffa52d07e260c2c25d3861810ea6157956c1793ad0c906284','2019-04-01 16:01:35.9254586+00:00',1);
			`,
		},
		storagenodedb.BandwidthDBName:      v26.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:     v26.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName: v26.DBStates[storagenodedb.DeprecatedInfoDBName],
	},
}
