// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/migrate"
)

var (
	// ErrMigrate is for tracking migration errors
	ErrMigrate = errs.Class("migrate")
)

// CreateTables is a method for creating all tables for database
func (db *DB) CreateTables() error {
	switch db.driver {
	case "postgres":
		schema, err := pgutil.ParseSchemaFromConnstr(db.source)
		if err != nil {
			return errs.New("error parsing schema: %+v", err)
		}
		if schema != "" {
			err = db.CreateSchema(schema)
			if err != nil {
				return errs.New("error creating schema: %+v", err)
			}
		}
		migration := db.PostgresMigration()
		return migration.Run(db.log.Named("migrate"))
	default:
		return migrate.Create("database", db.db)
	}
}

// CheckVersion confirms confirms the database is at the desired version
func (db *DB) CheckVersion() error {
	switch db.driver {
	case "postgres":
		migration := db.PostgresMigration()
		return migration.ValidateVersions(db.log)
	default:
		return nil
	}
}

// PostgresMigration returns steps needed for migrating postgres database.
func (db *DB) PostgresMigration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				// some databases may have already this done, although the version may not match
				DB:          db.db,
				Description: "Initial setup",
				Version:     0,
				Action:      migrate.SQL{},
			},
			{
				// some databases may have already this done, although the version may not match
				DB:          db.db,
				Description: "Adjust table naming",
				Version:     1,
				Action:      migrate.SQL{},
			},
			{
				// some databases may have already this done, although the version may not match
				DB:          db.db,
				Description: "Remove bucket infos",
				Version:     2,
				Action:      migrate.SQL{},
			},
			{
				// some databases may have already this done, although the version may not match
				DB:          db.db,
				Description: "Add certificates table",
				Version:     3,
				Action:      migrate.SQL{},
			},
			{
				// some databases may have already this done, although the version may not match
				DB:          db.db,
				Description: "Adjust users table",
				Version:     4,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add wallet column",
				Version:     5,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add bucket usage rollup table",
				Version:     6,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add index on bwagreements",
				Version:     7,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add registration_tokens table",
				Version:     8,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add new tables for tracking used serials, bandwidth and storage",
				Version:     9,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "users first_name to full_name, last_name to short_name",
				Version:     10,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "drops interval seconds from storage_rollups, renames x_storage_rollups to x_storage_tallies, adds fields to bucket_storage_tallies",
				Version:     11,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Merge overlay_cache_nodes into nodes table",
				Version:     12,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Change bucket_id to bucket_name and project_id",
				Version:     13,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add new Columns to store version information",
				Version:     14,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Default Node Type should be invalid",
				Version:     15,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add path to injuredsegment to prevent duplicates",
				Version:     16,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Fix audit and uptime ratios for new nodes",
				Version:     17,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Drops storagenode_storage_tally table, Renames accounting_raws to storagenode_storage_tally, and Drops data_type and created_at columns",
				Version:     18,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Added new table to store reset password tokens",
				Version:     19,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Adds pending_audits table, adds 'contained' column to nodes table",
				Version:     20,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add last_ip column and index",
				Version:     21,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Create new tables for free credits program",
				Version:     22,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Drops and recreates api key table to handle macaroons and adds revocation table",
				Version:     23,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add usage_limit column to projects table",
				Version:     24,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add disqualified column to nodes table",
				Version:     25,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add invitee_credit_in_gb and award_credit_in_gb columns, delete type and credit_in_cents columns",
				Version:     26,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Create value attribution table",
				Version:     27,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove agreements table",
				Version:     28,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add userpaymentinfos, projectpaymentinfos, projectinvoicestamps",
				Version:     29,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Alter value attribution table. Remove bucket_id. Add project_id and bucket_name as primary key",
				Version:     30,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add user_credit table",
				Version:     31,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Change type of disqualified column of nodes table to timestamp",
				Version:     32,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add alpha and beta columns for reputations",
				Version:     33,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove ratio columns from node reputations",
				Version:     34,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Fix reputations to preserve a baseline",
				Version:     35,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Update Last_IP column to be masked",
				Version:     36,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Update project_id column from 36 byte string based UUID to 16 byte UUID",
				Version:     37,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add bucket metadata table",
				Version:     38,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove disqualification flag for failing uptime checks",
				Version:     39,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add unique id for project payments. Add is_default property",
				Version:     40,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Move InjuredSegment path from string to bytes",
				Version:     41,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove num_redeemed column in offers table",
				Version:     42,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Set default offer for each offer type in offers table",
				Version:     43,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add index on InjuredSegments attempted column",
				Version:     44,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add partner id field to support OSPP",
				Version:     45,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add pending audit path",
				Version:     46,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Modify default offers configuration",
				Version:     47,
				Action:      migrate.SQL{},
			},
			{
				// This partial unique index enforces uniqueness among (id, offer_id) pairs for users that have signed up
				// but are not yet activated (credits_earned_in_cents=0).
				// Among users that are activated, uniqueness of (id, offer_id) pairs is not required or desirable.
				DB:          db.db,
				Description: "Create partial index for user_credits table",
				Version:     48,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add cascade to user_id for deleting an account",
				Version:     49,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Changing the primary key constraint",
				Version:     50,
				Action:      migrate.SQL{},
			},
			{
				// Creating owner_id column for project.
				// Removing projects without project members
				// And populating this column with first project member id
				DB:          db.db,
				Description: "Creating owner_id column for projects table",
				Version:     51,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove certRecords table",
				Version:     52,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add piece_count column to nodes table",
				Version:     53,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add Peer Identities table",
				Version:     54,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Added normalized_email column to users table",
				Version:     55,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add Graceful Exit tables and update nodes table",
				Version:     56,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add defaults to nodes table",
				Version:     57,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Remove timezone from Graceful Exit dates",
				Version:     58,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add table for storing stripe customers",
				Version:     59,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add coinpayments_transactions table",
				Version:     60,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add graceful exit success column to nodes table",
				Version:     61,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Alter graceful_exit_transfer_queue to have the piece_num as part of the primary key since it is possible for a node to have 2 pieces for a given segment.",
				Version:     62,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add payments update balance intents",
				Version:     63,
				Action: migrate.SQL{
					// version: 0
					`CREATE TABLE IF NOT EXISTS storagenode_storage_tallies (
							id bigserial NOT NULL,
							node_id bytea NOT NULL,
							interval_end_time timestamp with time zone NOT NULL,
							data_total double precision NOT NULL,
							PRIMARY KEY ( id )
						)`,
					`CREATE TABLE IF NOT EXISTS accounting_rollups (
							id bigserial NOT NULL,
							node_id bytea NOT NULL,
							start_time timestamp with time zone NOT NULL,
							put_total bigint NOT NULL,
							get_total bigint NOT NULL,
							get_audit_total bigint NOT NULL,
							get_repair_total bigint NOT NULL,
							put_repair_total bigint NOT NULL,
							at_rest_total double precision NOT NULL,
							PRIMARY KEY ( id )
						)`,
					`CREATE TABLE IF NOT EXISTS accounting_timestamps (
							name text NOT NULL,
							value timestamp with time zone NOT NULL,
							PRIMARY KEY ( name )
						)`,
					`CREATE TABLE IF NOT EXISTS injuredsegments (
							data bytea NOT NULL,
							path bytea NOT NULL,
							attempted timestamp,
							INDEX injuredsegments_attempted_index ( attempted ),
							PRIMARY KEY ( path )
						)`,
					`CREATE TABLE IF NOT EXISTS irreparabledbs (
							segmentpath bytea NOT NULL,
							segmentdetail bytea NOT NULL,
							pieces_lost_count bigint NOT NULL,
							seg_damaged_unix_sec bigint NOT NULL,
							repair_attempt_count bigint NOT NULL,
							PRIMARY KEY ( segmentpath )
						)`,
					`CREATE TABLE IF NOT EXISTS nodes (
							id bytea NOT NULL,
							audit_success_count bigint NOT NULL DEFAULT 0,
							total_audit_count bigint NOT NULL DEFAULT 0,
							uptime_success_count bigint NOT NULL,
							total_uptime_count bigint NOT NULL,
							wallet text NOT NULL DEFAULT '',
							email text NOT NULL DEFAULT '',
							address text NOT NULL DEFAULT '',
							protocol INTEGER NOT NULL DEFAULT 0,
							type INTEGER NOT NULL DEFAULT 0,
							free_bandwidth BIGINT NOT NULL DEFAULT -1,
							free_disk BIGINT NOT NULL DEFAULT -1,
							latency_90 BIGINT NOT NULL DEFAULT 0,
							last_contact_success TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch',
							last_contact_failure TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch',
							created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
							updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
							major bigint NOT NULL DEFAULT 0,
							minor bigint NOT NULL DEFAULT 0,
							patch bigint NOT NULL DEFAULT 0,
							hash TEXT NOT NULL DEFAULT '',
							timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT '0001-01-01 00:00:00+00',
							release bool NOT NULL DEFAULT FALSE,
							contained bool NOT NULL DEFAULT FALSE,
							last_net text NOT NULL DEFAULT '',
							disqualified timestamp,
							audit_reputation_alpha double precision NOT NULL DEFAULT 1,
							audit_reputation_beta double precision NOT NULL DEFAULT 0,
							uptime_reputation_alpha double precision NOT NULL DEFAULT 1,
							uptime_reputation_beta double precision NOT NULL DEFAULT 0,
							INDEX node_last_ip (last_net),
							piece_count BIGINT NOT NULL DEFAULT 0,
							exit_loop_completed_at TIMESTAMP,
							exit_initiated_at TIMESTAMP,
							exit_finished_at TIMESTAMP,
							exit_success boolean NOT NULL DEFAULT FALSE,
							PRIMARY KEY ( id )
						)`,
					`CREATE TABLE IF NOT EXISTS projects (
							id bytea NOT NULL,
							name text NOT NULL,
							description text NOT NULL,
							created_at timestamp with time zone NOT NULL,
							usage_limit bigint NOT NULL DEFAULT 0,
							partner_id BYTEA,
							owner_id BYTEA NOT NULL,
							PRIMARY KEY ( id )
						)`,
					`CREATE TABLE IF NOT EXISTS users (
							id bytea NOT NULL,
							full_name text NOT NULL,
							short_name text,
							email text NOT NULL,
							password_hash bytea NOT NULL,
							status integer NOT NULL,
							created_at timestamp with time zone NOT NULL,
							partner_id BYTEA,
							PRIMARY KEY ( id )
						)`,
					`CREATE TABLE IF NOT EXISTS api_keys (
						id bytea NOT NULL,
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						head bytea NOT NULL,
						name text NOT NULL,
						secret bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						partner_id BYTEA,
						normalized_email TEXT NOT NULL,
						PRIMARY KEY ( id ),
						UNIQUE ( head ),
						UNIQUE ( name, project_id )
						)`,
					`CREATE TABLE IF NOT EXISTS project_members (
							member_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
							project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
							created_at timestamp with time zone NOT NULL,
							PRIMARY KEY ( member_id, project_id )
						)`,

					// version: 1, bwagreements dropped
					// `ALTER TABLE bwagreements RENAME COLUMN storage_node TO storage_node_id;`,
					// `ALTER TABLE bwagreements ADD COLUMN uplink_id BYTEA;`,
					// `ALTER TABLE bwagreements ALTER COLUMN uplink_id SET NOT NULL;`,
					// `ALTER TABLE bwagreements DROP COLUMN data;`,

					// version: 2
					`DROP TABLE IF EXISTS bucket_infos CASCADE`,

					// version: 3
					// `CREATE TABLE IF NOT EXISTS certRecords (
					// 		publickey bytea NOT NULL,
					// 		id bytea NOT NULL,
					// 		update_at timestamp with time zone NOT NULL,
					// 		INDEX certrecord_id_update_at ( id, update_at ),
					// 		PRIMARY KEY ( publickey )
					// 	)`,

					// version: 4, combined w/version 0
					// `ALTER TABLE users ALTER COLUMN email SET NOT NULL;`,
					// `ALTER TABLE users ADD COLUMN status INTEGER;`,
					// todo: do we need this status set to something?
					// `UPDATE users SET status = 1;`,
					// `ALTER TABLE users ALTER COLUMN status SET NOT NULL;`,
					// `ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;`,

					// version: 5, combined w/version 0
					// `ALTER TABLE nodes ADD wallet TEXT;
					// 	ALTER TABLE nodes ADD email TEXT;
					// 	UPDATE nodes SET wallet = '';
					// 	UPDATE nodes SET email = '';
					// 	ALTER TABLE nodes ALTER COLUMN wallet SET NOT NULL;
					// 	ALTER TABLE nodes ALTER COLUMN email SET NOT NULL;`,

					// version: 6
					`CREATE TABLE IF NOT EXISTS bucket_usages (
							id bytea NOT NULL,
							bucket_id bytea NOT NULL,
							rollup_end_time timestamp with time zone NOT NULL,
							remote_stored_data bigint NOT NULL,
							inline_stored_data bigint NOT NULL,
							remote_segments integer NOT NULL,
							inline_segments integer NOT NULL,
							objects integer NOT NULL,
							metadata_size bigint NOT NULL,
							repair_egress bigint NOT NULL,
							get_egress bigint NOT NULL,
							audit_egress bigint NOT NULL,
							PRIMARY KEY ( id ),
						)`,

					// version: 7, bwagreements table dropped
					// `CREATE INDEX IF NOT EXISTS bwa_created_at ON bwagreements (created_at)`,

					// version: 8
					`CREATE TABLE IF NOT EXISTS registration_tokens (
							secret bytea NOT NULL,
							owner_id bytea,
							project_limit integer NOT NULL,
							created_at timestamp with time zone NOT NULL,
							PRIMARY KEY ( secret ),
							UNIQUE ( owner_id )
						)`,

					// version: 9
					`CREATE TABLE IF NOT EXISTS serial_numbers (
							id serial NOT NULL,
							serial_number bytea NOT NULL,
							bucket_id bytea NOT NULL,
							expires_at timestamp NOT NULL,
							PRIMARY KEY ( id )
						)`,
					`CREATE INDEX serial_numbers_expires_at_index ON serial_numbers ( expires_at )`,
					`CREATE UNIQUE INDEX serial_number_index ON serial_numbers ( serial_number )`,
					`CREATE TABLE IF NOT EXISTS used_serials (
							serial_number_id integer NOT NULL REFERENCES serial_numbers( id ) ON DELETE CASCADE,
							storage_node_id bytea NOT NULL,
							PRIMARY KEY ( serial_number_id, storage_node_id )
						)`,
					`CREATE TABLE IF NOT EXISTS storagenode_bandwidth_rollups (
							storagenode_id bytea NOT NULL,
							interval_start timestamp NOT NULL,
							interval_seconds integer NOT NULL,
							action integer NOT NULL,
							allocated bigint NOT NULL,
							settled bigint NOT NULL,
							PRIMARY KEY ( storagenode_id, interval_start, action )
						)`,
					`CREATE INDEX storagenode_id_interval_start_interval_seconds_index ON storagenode_bandwidth_rollups (
							storagenode_id,
							interval_start,
							interval_seconds
						)`,
					`CREATE TABLE IF NOT EXISTS bucket_bandwidth_rollups (
							bucket_name bytea NOT NULL,
							project_id bytea NOT NULL ,
							interval_start timestamp NOT NULL,
							interval_seconds integer NOT NULL,
							action integer NOT NULL,
							inline bigint NOT NULL,
							allocated bigint NOT NULL,
							settled bigint NOT NULL,
							INDEX bucket_name_project_id_interval_start_interval_seconds ( bucket_name, project_id, interval_start, interval_seconds ),
							PRIMARY KEY ( bucket_name, project_id, interval_start, action )
						)`,
					`CREATE INDEX bucket_id_interval_start_interval_seconds_index ON bucket_bandwidth_rollups (
							bucket_id,
							interval_start,
							interval_seconds
						)`,
					`CREATE TABLE IF NOT EXISTS bucket_storage_tallies (
							bucket_name bytea NOT NULL,
							project_id bytea NOT NULL ,
							interval_start timestamp NOT NULL,
							inline bigint NOT NULL,
							remote bigint NOT NULL,
							remote_segments_count integer NOT NULL DEFAULT 0,
							inline_segments_count integer NOT NULL DEFAULT 0,
							object_count integer NOT NULL DEFAULT 0,
							metadata_size bigint NOT NULL DEFAULT 0,
							PRIMARY KEY ( bucket_name, project_id, interval_start )
						)`,
					// combined with version 6
					// `ALTER TABLE bucket_usages DROP CONSTRAINT bucket_usages_rollup_end_time_bucket_id_key`,
					`CREATE UNIQUE INDEX bucket_id_rollup_end_time_index ON bucket_usages (
							bucket_id,
							rollup_end_time
						)`,

					// version: 10, combined with version 0
					// `ALTER TABLE users RENAME COLUMN first_name TO full_name;
					// ALTER TABLE users ALTER COLUMN last_name DROP NOT NULL;
					// ALTER TABLE users RENAME COLUMN last_name TO short_name;`,

					// version: 11, combined w/version 0
					// `ALTER TABLE storagenode_storage_rollups RENAME TO storagenode_storage_tallies`,
					// `ALTER TABLE bucket_storage_rollups RENAME TO bucket_storage_tallies`,
					// `ALTER TABLE storagenode_storage_tallies DROP COLUMN interval_seconds`,
					// `ALTER TABLE bucket_storage_tallies DROP COLUMN interval_seconds`,
					// `ALTER TABLE bucket_storage_tallies ADD remote_segments_count integer;
					// UPDATE bucket_storage_tallies SET remote_segments_count = 0;
					// ALTER TABLE bucket_storage_tallies ALTER COLUMN remote_segments_count SET NOT NULL;`,
					// `ALTER TABLE bucket_storage_tallies ADD inline_segments_count integer;
					// UPDATE bucket_storage_tallies SET inline_segments_count = 0;
					// ALTER TABLE bucket_storage_tallies ALTER COLUMN inline_segments_count SET NOT NULL;`,
					// `ALTER TABLE bucket_storage_tallies ADD object_count integer;
					// UPDATE bucket_storage_tallies SET object_count = 0;
					// ALTER TABLE bucket_storage_tallies ALTER COLUMN object_count SET NOT NULL;`,
					// `ALTER TABLE bucket_storage_tallies ADD metadata_size bigint;
					// UPDATE bucket_storage_tallies SET metadata_size = 0;
					// ALTER TABLE bucket_storage_tallies ALTER COLUMN metadata_size SET NOT NULL;`,

					// version: 12, combined w/ version 0
					// `ALTER TABLE nodes ADD address TEXT NOT NULL DEFAULT '';
					//  ALTER TABLE nodes ADD protocol INTEGER NOT NULL DEFAULT 0;
					//  ALTER TABLE nodes ADD type INTEGER NOT NULL DEFAULT 2;
					//  ALTER TABLE nodes ADD free_bandwidth BIGINT NOT NULL DEFAULT -1;
					//  ALTER TABLE nodes ADD free_disk BIGINT NOT NULL DEFAULT -1;
					//  ALTER TABLE nodes ADD latency_90 BIGINT NOT NULL DEFAULT 0;
					//  ALTER TABLE nodes ADD last_contact_success TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch';
					//  ALTER TABLE nodes ADD last_contact_failure TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch';`,
					// `DROP TABLE IF EXISTS overlay_cache_nodes CASCADE;`,

					// version: 13, combined w/version 0
					// `ALTER TABLE bucket_storage_tallies ADD project_id bytea;`,
					// `ALTER TABLE bucket_storage_tallies ALTER COLUMN project_id SET NOT NULL;`,
					// `ALTER TABLE bucket_storage_tallies RENAME COLUMN bucket_id TO bucket_name;`,
					// `ALTER TABLE bucket_storage_tallies DROP CONSTRAINT bucket_storage_rollups_pkey;`,
					// `ALTER TABLE bucket_storage_tallies ADD CONSTRAINT bucket_storage_tallies_pk PRIMARY KEY (bucket_name, project_id, interval_start);`,
					// `ALTER TABLE bucket_bandwidth_rollups ADD project_id bytea;`,
					// `ALTER TABLE bucket_bandwidth_rollups ALTER COLUMN project_id SET NOT NULL;`,
					// `ALTER TABLE bucket_bandwidth_rollups RENAME COLUMN bucket_id TO bucket_name;`,
					// `DROP INDEX IF EXISTS bucket_id_interval_start_interval_seconds_index;`,
					// `CREATE INDEX bucket_name_project_id_interval_start_interval_seconds ON bucket_bandwidth_rollups (
					// 	bucket_name,
					// 	project_id,
					// 	interval_start,
					// 	interval_seconds
					// 	);`,
					// `ALTER TABLE bucket_bandwidth_rollups DROP CONSTRAINT bucket_bandwidth_rollups_pkey;`,
					// `ALTER TABLE bucket_bandwidth_rollups ADD CONSTRAINT bucket_bandwidth_rollups_pk PRIMARY KEY (bucket_name, project_id, interval_start, action);`,

					// version: 14, combined w/version 0
					// `ALTER TABLE nodes ADD major bigint NOT NULL DEFAULT 0;
					// ALTER TABLE nodes ADD minor bigint NOT NULL DEFAULT 1;
					// ALTER TABLE nodes ADD patch bigint NOT NULL DEFAULT 0;
					// ALTER TABLE nodes ADD hash TEXT NOT NULL DEFAULT '';
					// ALTER TABLE nodes ADD timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch';
					// ALTER TABLE nodes ADD release bool NOT NULL DEFAULT FALSE;`,

					// version: 15, combined w/version 0
					// `ALTER TABLE nodes ALTER COLUMN type SET DEFAULT 0;`,

					// version: 16, combined w/version 0
					// `ALTER TABLE injuredsegments ADD path text;
					// ALTER TABLE injuredsegments RENAME COLUMN info TO data;
					// ALTER TABLE injuredsegments ADD attempted timestamp;
					// ALTER TABLE injuredsegments DROP CONSTRAINT IF EXISTS id_pkey;`,
					// `ALTER TABLE injuredsegments DROP COLUMN id;
					// ALTER TABLE injuredsegments ALTER COLUMN path SET NOT NULL;
					// ALTER TABLE injuredsegments ADD PRIMARY KEY (path);`,

					// version: 17
					// columns dropped in version 34
					// `UPDATE nodes SET audit_success_ratio = 1 WHERE total_audit_count = 0;
					// UPDATE nodes SET uptime_ratio = 1 WHERE total_uptime_count = 0;`,

					// version: 18, combined w/version 0
					// `DROP TABLE storagenode_storage_tallies CASCADE`,
					// `ALTER TABLE accounting_raws RENAME TO storagenode_storage_tallies`,
					// `ALTER TABLE storagenode_storage_tallies DROP COLUMN data_type`,
					// `ALTER TABLE storagenode_storage_tallies DROP COLUMN created_at`,

					// version: 19
					`CREATE TABLE IF NOT EXISTS reset_password_tokens (
						secret bytea NOT NULL,
						owner_id bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( secret ),
						UNIQUE ( owner_id )
					);`,

					// version: 20, combined w/version 0
					// `ALTER TABLE nodes ADD contained boolean;
					// UPDATE nodes SET contained = false;
					// ALTER TABLE nodes ALTER COLUMN contained SET NOT NULL;`,
					`CREATE TABLE IF NOT EXISTS pending_audits (
						node_id bytea NOT NULL,
						piece_id bytea NOT NULL,
						stripe_index bigint NOT NULL,
						share_size bigint NOT NULL,
						expected_share_hash bytea NOT NULL,
						reverify_count bigint NOT NULL,
						path bytea NOT NULL,
						PRIMARY KEY ( node_id )
					);`,

					// version: 21, combined w/version 0
					// `ALTER TABLE nodes ADD last_ip TEXT;
					// UPDATE nodes SET last_ip = '';
					// ALTER TABLE nodes ALTER COLUMN last_ip SET NOT NULL;
					// CREATE INDEX IF NOT EXISTS node_last_ip ON nodes (last_ip)`,

					// version: 22
					`CREATE TABLE IF NOT EXISTS offers (
						id serial NOT NULL,
						name text NOT NULL,
						description text NOT NULL,
						type integer NOT NULL,
						award_credit_in_cents integer NOT NULL DEFAULT 0,
						invitee_credit_in_cents integer NOT NULL DEFAULT 0,
						award_credit_duration_days integer,
						invitee_credit_duration_days integer,
						redeemable_cap integer,
						expires_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						status integer NOT NULL,
						PRIMARY KEY ( id )
					);`,

					// version: 23, combined w/version 0
					// `DROP TABLE api_keys CASCADE`,
					// `CREATE TABLE IF NOT EXISTS api_keys (
					// 	id bytea NOT NULL,
					// 	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
					// 	head bytea NOT NULL,
					// 	name text NOT NULL,
					// 	secret bytea NOT NULL,
					// 	created_at timestamp with time zone NOT NULL,
					// 	PRIMARY KEY ( id ),
					// 	UNIQUE ( head ),
					// 	UNIQUE ( name, project_id )
					// );`,

					// version: 24, all combined w/v0
					// `ALTER TABLE projects ADD usage_limit bigint NOT NULL DEFAULT 0;`,
					// // version: 25
					// `ALTER TABLE nodes ADD disqualified boolean NOT NULL DEFAULT false;`,
					// // version: 26
					// `ALTER TABLE offers DROP COLUMN credit_in_cents;`,
					// `ALTER TABLE offers ADD COLUMN award_credit_in_cents integer NOT NULL DEFAULT 0;`,
					// `ALTER TABLE offers ADD COLUMN invitee_credit_in_cents integer NOT NULL DEFAULT 0;`,
					// `ALTER TABLE offers ALTER COLUMN expires_at SET NOT NULL;`,

					// version: 27
					`CREATE TABLE IF NOT EXISTS value_attributions (
						bucket_name bytea NOT NULL,
						partner_id bytea NOT NULL,
						last_updated timestamp NOT NULL,
						project_id bytea NOT NULL,
						PRIMARY KEY (project_id, bucket_name)
						)`,
					// version: 28, combined w/v0
					// `DROP TABLE bwagreements`,

					// version: 29
					// dropped table in version 59
					// `CREATE TABLE IF NOT EXISTS user_payments (
					// 	user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
					// 	customer_id bytea NOT NULL,
					// 	created_at timestamp with time zone NOT NULL,
					// 	PRIMARY KEY ( user_id ),
					// 	UNIQUE ( customer_id )
					// );`,
					`CREATE TABLE IF NOT EXISTS project_invoice_stamps (
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						invoice_id bytea NOT NULL,
						start_date timestamp with time zone NOT NULL,
						end_date timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( project_id, start_date, end_date ),
						UNIQUE ( invoice_id )
					);`,

					// version: 30, combined w/table creation
					// `ALTER TABLE value_attributions DROP CONSTRAINT value_attributions_pkey;`,
					// `ALTER TABLE value_attributions ADD project_id bytea;`,
					// `UPDATE value_attributions SET project_id=SUBSTRING(bucket_id FROM 1 FOR 16);`,
					// `ALTER TABLE value_attributions ALTER COLUMN project_id SET NOT NULL;`,
					// `ALTER TABLE value_attributions RENAME COLUMN bucket_id TO bucket_name;`,
					// `UPDATE value_attributions SET bucket_name=SUBSTRING(bucket_name from 18);`,
					// `ALTER TABLE value_attributions ADD PRIMARY KEY (project_id, bucket_name);`,

					// version: 31
					`CREATE TABLE IF NOT EXISTS user_credits (
						id serial NOT NULL,
						user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						referred_by bytea REFERENCES users( id ) ON DELETE NO ACTION,
						offer_id integer NOT NULL REFERENCES offers( id ),
						credits_earned_in_cents integer NOT NULL,
						credits_used_in_cents integer NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						type text NOT NULL DEFAULT 'invalid',
						UNIQUE credits_earned_user_id_offer_id ( id, offer_id ),
						PRIMARY KEY ( id )
					);`,

					// version: 32
					// `ALTER TABLE nodes
					// 	ALTER COLUMN disqualified DROP DEFAULT,
					// 	ALTER COLUMN disqualified DROP NOT NULL,
					// 	ALTER COLUMN disqualified TYPE timestamp with time zone USING
					// 		CASE disqualified
					// 			WHEN true THEN TIMESTAMP WITH TIME ZONE '2019-06-15 00:00:00+00'
					// 			ELSE NULL
					// 		END`,

					// version: 33
					// `ALTER TABLE nodes ADD COLUMN audit_reputation_alpha double precision NOT NULL DEFAULT 1;`,
					// `ALTER TABLE nodes ADD COLUMN audit_reputation_beta double precision NOT NULL DEFAULT 0;`,
					// `ALTER TABLE nodes ADD COLUMN uptime_reputation_alpha double precision NOT NULL DEFAULT 1;`,
					// `ALTER TABLE nodes ADD COLUMN uptime_reputation_beta double precision NOT NULL DEFAULT 0;`,
					// // version: 34
					// `ALTER TABLE nodes DROP COLUMN audit_success_ratio;`,
					// `ALTER TABLE nodes DROP COLUMN uptime_ratio;`,

					// version: 35
					// `UPDATE nodes SET audit_reputation_alpha = GREATEST(audit_success_count, 50);`,
					// `UPDATE nodes SET audit_reputation_beta = total_audit_count - audit_success_count;`,
					// `UPDATE nodes SET uptime_reputation_alpha = GREATEST(uptime_success_count, 100);`,
					// `UPDATE nodes SET uptime_reputation_beta = total_uptime_count - uptime_success_count;`,

					// version: 36
					// `UPDATE nodes SET last_ip = host(network(set_masklen(last_ip::INET, 24))) WHERE last_ip <> '' AND family(last_ip::INET) = 4;`,
					// `UPDATE nodes SET last_ip = host(network(set_masklen(last_ip::INET, 64))) WHERE last_ip <> '' AND family(last_ip::INET) = 16;`,
					// `ALTER TABLE nodes RENAME last_ip TO last_net;`,

					// todo: confirm we dont need version 37

					// version: 38
					`CREATE TABLE IF NOT EXISTS bucket_metainfos (
						id bytea NOT NULL,
						project_id bytea NOT NULL REFERENCES projects( id ),
						name bytea NOT NULL,
						path_cipher integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						default_segment_size integer NOT NULL,
						default_encryption_cipher_suite integer NOT NULL,
						default_encryption_block_size integer NOT NULL,
						default_redundancy_algorithm integer NOT NULL,
						default_redundancy_share_size integer NOT NULL,
						default_redundancy_required_shares integer NOT NULL,
						default_redundancy_repair_shares integer NOT NULL,
						default_redundancy_optimal_shares integer NOT NULL,
						default_redundancy_total_shares integer NOT NULL,
						partner_id BYTEA,
						PRIMARY KEY ( id ),
						UNIQUE ( name, project_id )
					);`,
					// version: 39
					// `UPDATE nodes SET disqualified=NULL WHERE disqualified IS NOT NULL AND audit_reputation_alpha / (audit_reputation_alpha + audit_reputation_beta) >= 0.6;`,
					// version: 40
					// `DROP TABLE project_payments CASCADE`,
					// dropped in version 59
					// `CREATE TABLE IF NOT EXISTS project_payments (
					// 	id bytea NOT NULL,
					// 	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
					// 	payer_id bytea NOT NULL REFERENCES user_payments( user_id ) ON DELETE CASCADE,
					// 	payment_method_id bytea NOT NULL,
					// 	is_default boolean NOT NULL,
					// 	created_at timestamp with time zone NOT NULL,
					// 	PRIMARY KEY ( id )
					// );`,

					// version: 41, combined w/table creation
					// `ALTER TABLE injuredsegments RENAME COLUMN path TO path_old;`,
					// `ALTER TABLE injuredsegments ADD COLUMN path bytea;`,
					// `UPDATE injuredsegments SET path = decode(path_old, 'escape');`,
					// `ALTER TABLE injuredsegments ALTER COLUMN path SET NOT NULL;`,
					// `ALTER TABLE injuredsegments DROP COLUMN path_old;`,
					// `ALTER TABLE injuredsegments ADD CONSTRAINT injuredsegments_pk PRIMARY KEY (path);`,
					// // version: 42
					// `ALTER TABLE offers DROP num_redeemed;`,

					// version: 43,combined w/table creation
					// `ALTER TABLE offers
					// 	ALTER COLUMN redeemable_cap DROP NOT NULL,
					// 	ALTER COLUMN invitee_credit_duration_days DROP NOT NULL,
					// 	ALTER COLUMN award_credit_duration_days DROP NOT NULL
					// `,
					`INSERT INTO offers (
						name,
						description,
						award_credit_in_cents,
						invitee_credit_in_cents,
						expires_at,
						created_at,
						status,
						type )
					VALUES (
						'Default referral offer',
						'Is active when no other active referral offer',
						300,
						600,
						'2119-03-14 08:28:24.636949+00',
						'2019-07-14 08:28:24.636949+00',
						1,
						2
					),
					(
						'Default free credit offer',
						'Is active when no active free credit offer',
						300,
						0,
						'2119-03-14 08:28:24.636949+00',
						'2019-07-14 08:28:24.636949+00',
						1,
						1
					) ON CONFLICT DO NOTHING;`,

					// version: 44, added to table creation
					// `CREATE INDEX injuredsegments_attempted_index ON injuredsegments ( attempted );`,

					// version: 45,  combined w/table creation
					// `ALTER TABLE projects ADD COLUMN partner_id BYTEA`,
					// `ALTER TABLE users ADD COLUMN partner_id BYTEA`,
					// `ALTER TABLE api_keys ADD COLUMN partner_id BYTEA`,
					// `ALTER TABLE bucket_metainfos ADD COLUMN partner_id BYTEA`,

					// version: 46, added to table creation
					// `DELETE FROM pending_audits;`, // clearing pending_audits is the least-bad choice to deal with the added 'path' column
					// `ALTER TABLE pending_audits ADD COLUMN path bytea NOT NULL;`,
					// `UPDATE nodes SET contained = false;`,

					// version: 47, don't keep updates
					// `UPDATE offers SET
					// 	award_credit_duration_days = 365,
					// 	invitee_credit_duration_days = 14
					// 	WHERE type=2 AND status=1 AND id=1`,
					// `UPDATE offers SET
					// 	invitee_credit_duration_days = 14,
					// 	award_credit_duration_days = NULL,
					// 	award_credit_in_cents = 0,
					// 	invitee_credit_in_cents = 300
					// 	WHERE type=1 AND status=1 AND id=2;`,

					// version: 48
					// added this to table createion, but w/o WHERE clause
					// todo: is that ok? ^
					// `CREATE UNIQUE INDEX credits_earned_user_id_offer_id ON user_credits (id, offer_id)
					// WHERE credits_earned_in_cents=0;`,

					// version: 49, added to table creation
					// `ALTER TABLE user_credits DROP CONSTRAINT user_credits_referred_by_fkey;
					// ALTER TABLE user_credits ADD CONSTRAINT user_credits_referred_by_fkey
					// 	FOREIGN KEY (referred_by) REFERENCES users(id) ON DELETE SET NULL;
					// ALTER TABLE user_credits DROP CONSTRAINT user_credits_user_id_fkey;
					// ALTER TABLE user_credits ADD CONSTRAINT user_credits_user_id_fkey
					// 	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
					// ALTER TABLE user_credits ADD COLUMN type text;
					// UPDATE user_credits SET type='invalid';
					// ALTER TABLE user_credits ALTER COLUMN type SET NOT NULL;`,

					// version: 50, table dropped version 52
					// `ALTER TABLE certRecords DROP CONSTRAINT certrecords_pkey;
					// ALTER TABLE certRecords ADD CONSTRAINT certrecords_pkey PRIMARY KEY (publickey);
					// CREATE INDEX certrecord_id_update_at ON certRecords ( id, update_at );`,

					// version: 51
					// `ALTER TABLE projects ADD COLUMN owner_id BYTEA;`,
					// `ALTER TABLE projects ALTER COLUMN owner_id SET NOT NULL;`,

					// version: 52
					// `DROP TABLE certRecords CASCADE`,

					// version: 53
					// `ALTER TABLE nodes ADD piece_count BIGINT NOT NULL DEFAULT 0;`,

					// version: 54
					`CREATE TABLE IF NOT EXISTS peer_identities (
						node_id bytea NOT NULL,
						leaf_serial_number bytea NOT NULL,
						chain bytea NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( node_id )
					);`,

					// // version: 55
					// `ALTER TABLE users ADD normalized_email TEXT;`,
					// `UPDATE users SET normalized_email=UPPER(email);`,
					// `ALTER TABLE users ALTER COLUMN normalized_email SET NOT NULL;`,

					// // version: 56
					// `ALTER TABLE nodes ADD COLUMN exit_loop_completed_at timestamp with time zone;`,
					// `ALTER TABLE nodes ADD COLUMN exit_initiated_at timestamp with time zone;`,
					// `ALTER TABLE nodes ADD COLUMN exit_finished_at timestamp with time zone;`,

					`CREATE TABLE IF NOT EXISTS graceful_exit_progress (
						node_id bytea NOT NULL,
						bytes_transferred bigint NOT NULL,
						updated_at timestamp NOT NULL,
						pieces_transferred bigint NOT NULL DEFAULT 0,
						pieces_failed bigint NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id )
					);`,
					`CREATE TABLE IF NOT EXISTS graceful_exit_transfer_queue (
						node_id bytea NOT NULL,
						path bytea NOT NULL,
						piece_num integer NOT NULL,
						durability_ratio double precision NOT NULL,
						queued_at timestamp NOT NULL,
						requested_at timestamp,
						last_failed_at timestamp,
						last_failed_code integer,
						failed_count integer,
						finished_at timestamp,
						PRIMARY KEY ( node_id, path, piece_num )
					);`,

					// version: 57
					// `ALTER TABLE nodes ALTER COLUMN contained SET DEFAULT false;`,
					// `ALTER TABLE nodes ALTER COLUMN piece_count SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN major SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN minor SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN audit_success_count SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN total_audit_count SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN patch SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN hash SET DEFAULT '';`,
					// `ALTER TABLE nodes ALTER COLUMN release SET DEFAULT false;`,
					// `ALTER TABLE nodes ALTER COLUMN latency_90 SET DEFAULT 0;`,
					// `ALTER TABLE nodes ALTER COLUMN timestamp SET DEFAULT '0001-01-01 00:00:00+00';`,
					// `ALTER TABLE nodes ALTER COLUMN created_at SET DEFAULT current_timestamp;`,
					// `ALTER TABLE nodes ALTER COLUMN updated_at SET DEFAULT current_timestamp;`,

					// // version: 58
					// `ALTER TABLE nodes ALTER COLUMN exit_initiated_at TYPE timestamp;`,
					// `ALTER TABLE nodes ALTER COLUMN exit_loop_completed_at TYPE timestamp;`,
					// `ALTER TABLE nodes ALTER COLUMN exit_finished_at TYPE timestamp;`,
					// how to add UTC to type
					// `UPDATE graceful_exit_progress set updated_at = TIMEZONE('UTC', updated_at);`,
					// `ALTER TABLE graceful_exit_progress ADD COLUMN pieces_transferred bigint NOT NULL DEFAULT 0;`,
					// `ALTER TABLE graceful_exit_progress ADD COLUMN pieces_failed bigint NOT NULL DEFAULT 0;`,
					// `ALTER TABLE graceful_exit_progress ALTER COLUMN updated_at TYPE timestamp;`,
					// `ALTER TABLE graceful_exit_transfer_queue ALTER COLUMN queued_at TYPE timestamp;`,
					// `ALTER TABLE graceful_exit_transfer_queue ALTER COLUMN requested_at TYPE timestamp;`,
					// `ALTER TABLE graceful_exit_transfer_queue ALTER COLUMN last_failed_at TYPE timestamp;`,
					// `ALTER TABLE graceful_exit_transfer_queue ALTER COLUMN finished_at TYPE timestamp;`,

					// version: 59
					// `DROP TABLE project_payments CASCADE`,
					// `DROP TABLE user_payments CASCADE`,
					`CREATE TABLE IF NOT EXISTS stripe_customers (
						user_id bytea NOT NULL,
						customer_id text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( user_id ),
						UNIQUE ( customer_id )
						);`,

					// version: 60
					`CREATE TABLE IF NOT EXISTS coinpayments_transactions (
						id text NOT NULL,
						user_id bytea NOT NULL,
						address text NOT NULL,
						amount bytea NOT NULL,
						received bytea NOT NULL,
						status integer NOT NULL,
						key text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,

					// version: 61
					// `ALTER TABLE nodes ADD COLUMN exit_success boolean NOT NULL DEFAULT FALSE`,
					// // version: 62
					// `ALTER TABLE graceful_exit_transfer_queue DROP CONSTRAINT graceful_exit_transfer_queue_pkey;`,
					// `ALTER TABLE graceful_exit_transfer_queue ADD PRIMARY KEY ( node_id, path, piece_num );`,

					// version: 63
					`CREATE TABLE IF NOT EXISTS stripecoinpayments_apply_balance_intents (
						tx_id text NOT NULL REFERENCES coinpayments_transactions( id ) ON DELETE CASCADE,
						state integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( tx_id )
					);`,
				},
			},
		},
	}
}

func postgresHasColumn(tx *sql.Tx, table, column string) (bool, error) {
	var columnName string
	err := tx.QueryRow(`
		SELECT column_name FROM information_schema.COLUMNS
			WHERE table_schema = CURRENT_SCHEMA
				AND table_name = $1
				AND column_name = $2
		`, table, column).Scan(&columnName)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, ErrMigrate.Wrap(err)
	}

	return columnName == column, nil
}

func postgresColumnNullability(tx *sql.Tx, table, column string) (bool, error) {
	var nullability string
	err := tx.QueryRow(`
		SELECT is_nullable FROM information_schema.COLUMNS
			WHERE table_schema = CURRENT_SCHEMA
				AND table_name = $1
				AND column_name = $2
		`, table, column).Scan(&nullability)
	if err != nil {
		return false, ErrMigrate.Wrap(err)
	}
	return nullability == "YES", nil
}
