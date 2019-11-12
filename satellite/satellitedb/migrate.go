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
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Removing unused bucket_usages table",
				Version:     64,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Add stripecoinpayments_invoice_project_records",
				Version:     65,
				Action:      migrate.SQL{},
			},
			{
				DB:          db.db,
				Description: "Alter graceful_exit_transfer_queue to add root_piece_id.",
				Version:     66,
				Action: migrate.SQL{
					// version: 0
					`CREATE TABLE storagenode_storage_tallies (
						id bigserial NOT NULL,
						node_id bytea NOT NULL,
						interval_end_time timestamp with time zone NOT NULL,
						data_total double precision NOT NULL,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE accounting_rollups (
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
					);`,

					`CREATE TABLE accounting_timestamps (
						name text NOT NULL,
						value timestamp with time zone NOT NULL,
						PRIMARY KEY ( name )
					);`,

					`CREATE TABLE injuredsegments (
						data bytea NOT NULL,
						path bytea NOT NULL,
						attempted timestamp,
						INDEX injuredsegments_attempted_index ( attempted ),
						PRIMARY KEY ( path )
					);`,

					`CREATE TABLE irreparabledbs (
						segmentpath bytea NOT NULL,
						segmentdetail bytea NOT NULL,
						pieces_lost_count bigint NOT NULL,
						seg_damaged_unix_sec bigint NOT NULL,
						repair_attempt_count bigint NOT NULL,
						PRIMARY KEY ( segmentpath )
					);`,

					`CREATE TABLE nodes (
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
						disqualified timestamp with time zone,
						audit_reputation_alpha double precision NOT NULL DEFAULT 1,
						audit_reputation_beta double precision NOT NULL DEFAULT 0,
						uptime_reputation_alpha double precision NOT NULL DEFAULT 1,
						uptime_reputation_beta double precision NOT NULL DEFAULT 0,
						piece_count bigint NOT NULL DEFAULT 0,
						exit_loop_completed_at TIMESTAMP,
						exit_initiated_at TIMESTAMP,
						exit_finished_at TIMESTAMP,
						exit_success boolean NOT NULL DEFAULT FALSE,
						INDEX node_last_ip (last_net),
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE projects (
						id bytea NOT NULL,
						name text NOT NULL,
						description text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						usage_limit bigint NOT NULL DEFAULT 0,
						partner_id bytea,
						owner_id bytea NOT NULL,
						PRIMARY KEY ( id )
					);`,

					// todo: do we need users.status to have a default
					// see version: 4 migration:
					// UPDATE users SET status = ` + strconv.Itoa(int(console.Active)) + `;
					`CREATE TABLE users (
						id bytea NOT NULL,
						full_name text NOT NULL,
						short_name text,
						email text NOT NULL,
						password_hash bytea NOT NULL,
						status integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						partner_id bytea,
						normalized_email text NOT NULL,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE api_keys (
						id bytea NOT NULL,
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						head bytea NOT NULL UNIQUE,
						name text NOT NULL,
						secret bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						partner_id bytea,
						PRIMARY KEY ( id ),
						UNIQUE ( name, project_id )
					);`,

					`CREATE TABLE project_members (
						member_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( member_id, project_id )
					);`,

					// version: 8
					`CREATE TABLE registration_tokens (
						secret bytea NOT NULL,
						owner_id bytea UNIQUE,
						project_limit integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( secret )
					);`,

					// version: 9
					`CREATE TABLE serial_numbers (
						id serial NOT NULL,
						serial_number bytea NOT NULL UNIQUE,
						bucket_id bytea NOT NULL,
						expires_at timestamp NOT NULL,
						PRIMARY KEY ( id ),
						INDEX serial_numbers_expires_at_index ( expires_at )
					);`,

					`CREATE TABLE used_serials (
						serial_number_id integer NOT NULL REFERENCES serial_numbers( id ) ON DELETE CASCADE,
						storage_node_id bytea NOT NULL,
						PRIMARY KEY ( serial_number_id, storage_node_id )
					);`,

					`CREATE TABLE storagenode_bandwidth_rollups (
						storagenode_id bytea NOT NULL,
						interval_start timestamp NOT NULL,
						interval_seconds integer NOT NULL,
						action integer NOT NULL,
						allocated bigint NOT NULL,
						settled bigint NOT NULL,
						PRIMARY KEY ( storagenode_id, interval_start, action ),
						INDEX storagenode_id_interval_start_interval_seconds_index ( storagenode_id, interval_start, interval_seconds )
					);`,

					`CREATE TABLE bucket_bandwidth_rollups (
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
					);`,

					`CREATE TABLE bucket_storage_tallies (
						bucket_name bytea NOT NULL,
						project_id bytea NOT NULL,
						interval_start timestamp NOT NULL,
						inline bigint NOT NULL,
						remote bigint NOT NULL,
						remote_segments_count integer NOT NULL DEFAULT 0,
						inline_segments_count integer NOT NULL DEFAULT 0,
						object_count integer NOT NULL DEFAULT 0,
						metadata_size bigint NOT NULL DEFAULT 0,
						PRIMARY KEY ( bucket_name, project_id, interval_start )
					);`,

					// version: 19
					`CREATE TABLE reset_password_tokens (
						secret bytea NOT NULL,
						owner_id bytea NOT NULL UNIQUE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( secret )
					);`,

					// version: 20
					`CREATE TABLE pending_audits (
						node_id bytea NOT NULL,
						piece_id bytea NOT NULL,
						stripe_index bigint NOT NULL,
						share_size bigint NOT NULL,
						expected_share_hash bytea NOT NULL,
						reverify_count bigint NOT NULL,
						path bytea NOT NULL,
						PRIMARY KEY ( node_id )
					);`,

					// version: 22
					`CREATE TABLE offers (
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

					// version: 27
					`CREATE TABLE value_attributions (
						bucket_name bytea NOT NULL,
						partner_id bytea NOT NULL,
						last_updated timestamp NOT NULL,
						project_id bytea NOT NULL,
						PRIMARY KEY (project_id, bucket_name)
					);`,

					`CREATE TABLE project_invoice_stamps (
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						invoice_id bytea NOT NULL UNIQUE,
						start_date timestamp with time zone NOT NULL,
						end_date timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( project_id, start_date, end_date )
					);`,

					// version: 31
					`CREATE TABLE user_credits (
						id serial NOT NULL,
						user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						referred_by bytea REFERENCES users( id ) ON DELETE NO ACTION,
						offer_id integer NOT NULL REFERENCES offers( id ),
						credits_earned_in_cents integer NOT NULL,
						credits_used_in_cents integer NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						type text NOT NULL DEFAULT 'invalid',
						UNIQUE ( id, offer_id ),
						PRIMARY KEY ( id )
					);`,

					// version: 38
					`CREATE TABLE bucket_metainfos (
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
						partner_id bytea,
						PRIMARY KEY ( id ),
						UNIQUE ( name, project_id )
					);`,

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

					// version: 54
					`CREATE TABLE peer_identities (
						node_id bytea NOT NULL,
						leaf_serial_number bytea NOT NULL,
						chain bytea NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( node_id )
					);`,

					`CREATE TABLE graceful_exit_progress (
						node_id bytea NOT NULL,
						bytes_transferred bigint NOT NULL,
						updated_at timestamp NOT NULL,
						pieces_transferred bigint NOT NULL DEFAULT 0,
						pieces_failed bigint NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id )
					);`,

					`CREATE TABLE graceful_exit_transfer_queue (
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
						root_piece_id bytea,
						PRIMARY KEY ( node_id, path, piece_num )
					);`,

					// version: 59
					`CREATE TABLE stripe_customers (
						user_id bytea NOT NULL,
						customer_id text NOT NULL UNIQUE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( user_id )
					);`,

					// version: 60
					`CREATE TABLE coinpayments_transactions (
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

					// version: 63
					`CREATE TABLE stripecoinpayments_apply_balance_intents (
						tx_id text NOT NULL REFERENCES coinpayments_transactions( id ) ON DELETE CASCADE,
						state integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( tx_id )
					);`,

					// version: 65
					`CREATE TABLE stripecoinpayments_invoice_project_records (
						id bytea NOT NULL,
						project_id bytea NOT NULL,
						storage double precision NOT NULL,
						egress bigint NOT NULL,
						objects bigint NOT NULL,
						period_start timestamp with time zone NOT NULL,
						period_end timestamp with time zone NOT NULL,
						state integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id ),
						UNIQUE ( project_id, period_start, period_end )
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
