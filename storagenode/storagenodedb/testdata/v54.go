// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import "storj.io/storj/storagenode/storagenodedb"

var v54 = MultiDBState{
	Version: 54,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName: v53.DBStates[storagenodedb.UsedSerialsDBName],
		storagenodedb.StorageUsageDBName: &DBState{
			SQL: `
				CREATE TABLE storage_usage (
					timestamp TIMESTAMP NOT NULL,
					satellite_id BLOB NOT NULL,
					at_rest_total REAL NOT NULL,
					interval_end_time TIMESTAMP NOT NULL,
					PRIMARY KEY (timestamp, satellite_id)
				);
				INSERT INTO storage_usage (satellite_id,                                                        at_rest_total,  interval_end_time,           timestamp) VALUES
										  (X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000', 5.0,            '2019-07-19 20:00:00+00:00', '2019-07-19 20:00:00+00:00');
			`,
		},
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
				INSERT INTO reputation (satellite_id,														 audit_success_count, audit_total_count, audit_reputation_alpha, audit_reputation_beta, audit_reputation_score, audit_unknown_reputation_alpha, audit_unknown_reputation_beta, audit_unknown_reputation_score, online_score, audit_history, disqualified_at,             updated_at,                  suspended_at, offline_suspended_at, offline_under_review_at, vetted_at,                   joined_at) VALUES
									   (X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000', 1,                   1,                 1.0,					 1.0,					1.0,					1.0,							1.0,						   1.0,							   1.0,			 NULL,			'2019-07-19 20:00:00+00:00', '2019-08-23 20:00:00+00:00', NULL,			NULL,				  NULL,					   NULL,						'1970-01-01 00:00:00+00:00'),
									   (X'953fdf144a088a4116a1f6acfc8475c78278c018849db050d894a89572e56d00', 1,                   1,                 1.0,                    1.0,                   1.0,                    1.0,                            1.0,                           1.0,                            1.0,          NULL,          '2019-07-19 20:00:00+00:00', '2019-08-23 20:00:00+00:00', NULL,         NULL,                 NULL,                    '2019-06-25 20:00:00+00:00', '1970-01-01 00:00:00+00:00'),
									   (X'1a438a44e3cc9ab9faaacc1c034339f0ebec05f310f0ba270414dac753882f00', 1,                   1,                 1.0,                    1.0,                   1.0,                    1.0,                            1.0,                           1.0,                            1.0,          NULL,          NULL,                        '2019-08-23 20:00:00+00:00', NULL,         NULL,                 NULL,                    NULL,                        '1970-01-01 00:00:00+00:00');
			`,
		},
		storagenodedb.PieceSpaceUsedDBName:  v53.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v53.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v53.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v53.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v53.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName: &DBState{
			SQL: `
				CREATE TABLE satellites (
					node_id BLOB NOT NULL,
					address TEXT,
					added_at TIMESTAMP NOT NULL,
					status INTEGER NOT NULL,
					PRIMARY KEY (node_id)
				);
				CREATE TABLE satellite_exit_progress (
					satellite_id BLOB NOT NULL,
					initiated_at TIMESTAMP,
					finished_at TIMESTAMP,
					starting_disk_usage INTEGER NOT NULL,
					bytes_deleted INTEGER NOT NULL,
					completion_receipt BLOB,
					FOREIGN KEY (satellite_id) REFERENCES satellites (node_id)
				);
				INSERT INTO satellites (node_id, 															 added_at, 					  status) VALUES
									   (X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000', '2019-09-10 20:00:00+00:00', 0);
				INSERT INTO satellite_exit_progress VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000','2019-09-10 20:00:00+00:00', null, 100, 0, null);
			`,
		},
		storagenodedb.DeprecatedInfoDBName: v53.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:  v53.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:     v53.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:        v53.DBStates[storagenodedb.PricingDBName],
		storagenodedb.APIKeysDBName:        v53.DBStates[storagenodedb.APIKeysDBName],
	},
}
