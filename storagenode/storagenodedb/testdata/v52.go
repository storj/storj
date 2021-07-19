// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v52 = MultiDBState{
	Version: 52,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:  v51.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName: v51.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName: &DBState{
			SQL: `
				-- table to store nodestats cache
				CREATE TABLE reputation (
					satellite_id BLOB NOT NULL,
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
					vetted_at TIMESTAMP,
					joined_at TIMESTAMP NOT NULL,
					PRIMARY KEY (satellite_id)
				);
				INSERT INTO reputation VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1,1.0,1.0,1.0,1.0,1.0,1.0,1.0,NULL,'2019-07-19 20:00:00+00:00','2019-08-23 20:00:00+00:00',NULL,NULL,NULL,NULL,'1970-01-01 00:00:00+00:00');
			`,
			NewData: `
				INSERT INTO reputation (satellite_id,														 audit_success_count, audit_total_count, audit_reputation_alpha, audit_reputation_beta, audit_reputation_score, audit_unknown_reputation_alpha, audit_unknown_reputation_beta, audit_unknown_reputation_score, online_score, audit_history, disqualified_at,             updated_at,                  suspended_at, offline_suspended_at, offline_under_review_at, vetted_at,                   joined_at) VALUES
									   (X'953fdf144a088a4116a1f6acfc8475c78278c018849db050d894a89572e56d00', 1,                   1,                 1.0,                    1.0,                   1.0,                    1.0,                            1.0,                           1.0,                            1.0,          NULL,          '2019-07-19 20:00:00+00:00', '2019-08-23 20:00:00+00:00', NULL,         NULL,                 NULL,                    '2019-06-25 20:00:00+00:00', '1970-01-01 00:00:00+00:00'),
									   (X'1a438a44e3cc9ab9faaacc1c034339f0ebec05f310f0ba270414dac753882f00', 1,                   1,                 1.0,                    1.0,                   1.0,                    1.0,                            1.0,                           1.0,                            1.0,          NULL,          NULL,                        '2019-08-23 20:00:00+00:00', NULL,         NULL,                 NULL,                    NULL,                        '1970-01-01 00:00:00+00:00');
			`,
		},
		storagenodedb.PieceSpaceUsedDBName:  v51.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v51.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v51.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v51.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v51.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v51.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v51.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v51.DBStates[storagenodedb.NotificationsDBName],
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
			-- distributed has been updated for the periods < 2020-12.
			INSERT INTO paystubs (period,    satellite_id, created_at,                    codes, usage_at_rest, usage_get, usage_put, usage_get_repair, usage_put_repair, usage_get_audit, comp_at_rest, comp_get, comp_put, comp_get_repair, comp_put_repair, comp_get_audit, surge_percent, held, owed, disposed, paid, distributed) VALUES
			                     ('2020-10', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   100,           200,       300,       400,              500,              600,             700,          800,      900,      1000,            1100,            1200,           1300,          1400, 1500, 1600,     1700, 1700),
			                     ('2020-11', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   101,           201,       301,       401,              501,              601,             701,          801,      901,      1010,            1101,            1201,           1301,          1401, 1501, 1601,     1701, 1701),
			                     ('2020-12', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   102,           202,       302,       402,              502,              602,             702,          802,      902,      1020,            1102,            1202,           1302,          1402, 1502, 1602,     1702, 0),
			                     ('2021-01', 'foo',        '2020-04-07T00:00:00.000000Z', 'X',   103,           203,       303,       403,              503,              603,             703,          803,      903,      1030,            1103,            1203,           1303,          1403, 1503, 1603,     1703, 0);
			`,
		},
		storagenodedb.PricingDBName: v51.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName: v51.DBStates[storagenodedb.APIKeysDBName],
	},
}
