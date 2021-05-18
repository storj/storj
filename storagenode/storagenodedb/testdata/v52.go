// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v52 = MultiDBState{
	Version: 52,
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
					distributed bigint NOT NULL,
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
				);

				INSERT INTO paystubs (period,    satellite_id, created_at,                    codes, usage_at_rest, usage_get, usage_put, usage_get_repair, usage_put_repair, usage_get_audit, comp_at_rest, comp_get, comp_put, comp_get_repair, comp_put_repair, comp_get_audit, surge_percent, held, owed, disposed, paid, distributed) VALUES
					('2020-10', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   100,           200,       300,       400,              500,              600,             700,          800,      900,      1000,            1100,            1200,           1300,          1400, 1500, 1600,     1700, 1700),
					('2020-11', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   101,           201,       301,       401,              501,              601,             701,          801,      901,      1010,            1101,            1201,           1301,          1401, 1501, 1601,     1701, 1701),
					('2020-12', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   102,           202,       302,       402,              502,              602,             702,          802,      902,      1020,            1102,            1202,           1302,          1402, 1502, 1602,     1702, 0),
					('2021-01', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   103,           203,       303,       403,              503,              603,             703,          803,      903,      1030,            1103,            1203,           1303,          1403, 1503, 1603,     1703, 0)
				`,
		},
		storagenodedb.PricingDBName: v47.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName: v47.DBStates[storagenodedb.APIKeysDBName],
		storagenodedb.PlannedDowntimeDBName: &DBState{
			SQL: `
				-- table to hold planned downtime data
				CREATE TABLE planned_downtime (
					id BLOB UNIQUE NOT NULL,
					start TIMESTAMP NOT NULL,
					end TIMESTAMP NOT NULL,
					scheduled_at TIMESTAMP NOT NULL
				);`,
		},
	},
}
