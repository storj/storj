// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

//go:generate go run migrate_gen.go

var (
	// ErrMigrate is for tracking migration errors.
	ErrMigrate = errs.Class("migrate")
	// ErrMigrateMinVersion is for migration min version errors.
	ErrMigrateMinVersion = errs.Class("migrate min version")
)

// MigrateToLatest migrates the database to the latest version.
func (db *satelliteDB) MigrateToLatest(ctx context.Context) error {
	// First handle the idiosyncrasies of postgres and cockroach migrations. Postgres
	// will need to create any schemas specified in the search path, and cockroach
	// will need to create the database it was told to connect to. These things should
	// not really be here, and instead should be assumed to exist.
	// This is tracked in jira ticket SM-200
	switch db.impl {
	case dbutil.Postgres:
		schema, err := pgutil.ParseSchemaFromConnstr(db.source)
		if err != nil {
			return errs.New("error parsing schema: %+v", err)
		}

		if schema != "" {
			err = pgutil.CreateSchema(ctx, db, schema)
			if err != nil {
				return errs.New("error creating schema: %+v", err)
			}
		}

	case dbutil.Cockroach:
		var dbName string
		if err := db.QueryRowContext(ctx, `SELECT current_database();`).Scan(&dbName); err != nil {
			return errs.New("error querying current database: %+v", err)
		}

		_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s;`,
			pgutil.QuoteIdentifier(dbName)))
		if err != nil {
			return errs.Wrap(err)
		}
	case dbutil.Spanner:
		// nothing to do here at the moment
	default:
		return Error.New("unsupported database: %v", db.impl)
	}

	switch db.impl {
	case dbutil.Postgres, dbutil.Cockroach, dbutil.Spanner:
		migration := db.ProductionMigration()
		// since we merged migration steps 0-69, the current db version should never be
		// less than 69 unless the migration hasn't run yet
		const minDBVersion = 69
		dbVersion, err := migration.CurrentVersion(ctx, db.log, db.DB)
		if err != nil {
			return errs.New("error current version: %+v", err)
		}
		if dbVersion > -1 && dbVersion < minDBVersion {
			return ErrMigrateMinVersion.New("current database version is %d, it shouldn't be less than the min version %d",
				dbVersion, minDBVersion,
			)
		}

		return migration.Run(ctx, db.log.Named("migrate"))
	default:
		return migrate.Create(ctx, "database", db.DB)
	}
}

// TestMigrateToLatest is a method for creating all tables for database for testing.
func (db *satelliteDBTesting) TestMigrateToLatest(ctx context.Context) error {
	switch db.impl {
	case dbutil.Postgres:
		schema, err := pgutil.ParseSchemaFromConnstr(db.source)
		if err != nil {
			return ErrMigrateMinVersion.New("error parsing schema: %+v", err)
		}

		if schema != "" {
			err = pgutil.CreateSchema(ctx, db, schema)
			if err != nil {
				return ErrMigrateMinVersion.New("error creating schema: %+v", err)
			}
		}

	case dbutil.Cockroach:
		var dbName string
		if err := db.QueryRowContext(ctx, `SELECT current_database();`).Scan(&dbName); err != nil {
			return ErrMigrateMinVersion.New("error querying current database: %+v", err)
		}

		_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s;`, pgutil.QuoteIdentifier(dbName)))
		if err != nil {
			return ErrMigrateMinVersion.Wrap(err)
		}

	case dbutil.Spanner:
		// nothing to do here
	default:
		return Error.New("unsupported database: %v", db.impl)
	}

	switch db.impl {
	case dbutil.Postgres, dbutil.Cockroach, dbutil.Spanner:
		migration := db.ProductionMigration()

		dbVersion, err := migration.CurrentVersion(ctx, db.log, db.DB)
		if err != nil {
			return ErrMigrateMinVersion.Wrap(err)
		}

		testMigration := db.TestMigration()
		if dbVersion != -1 && dbVersion != testMigration.Steps[0].Version {
			return ErrMigrateMinVersion.New("the database must be empty, or be on the latest version (%d)", dbVersion)
		}

		return testMigration.Run(ctx, db.log.Named("migrate"))
	default:
		return migrate.Create(ctx, "database", db.DB)
	}
}

// CheckVersion confirms the database is at the desired version.
func (db *satelliteDB) CheckVersion(ctx context.Context) error {
	switch db.impl {
	case dbutil.Postgres, dbutil.Cockroach, dbutil.Spanner:
		migration := db.ProductionMigration()
		return migration.ValidateVersions(ctx, db.log)

	default:
		return nil
	}
}

// TestMigration returns steps needed for migrating test postgres database.
func (db *satelliteDB) TestMigration() *migrate.Migration {
	return db.testMigration()
}

// ProductionMigration returns steps needed for migrating the satellitedb database.
func (db *satelliteDB) ProductionMigration() *migrate.Migration {
	if db.DB.Name() == "spanner" {
		return db.productionMigrationSpanner()
	}
	return db.productionMigrationPostgres()
}

// productionMigrationSpanner returns steps needed for migrating the spanner database.
func (db *satelliteDB) productionMigrationSpanner() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Initial setup",
				Version:     283,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS account_freeze_events (
						user_id BYTES(MAX) NOT NULL,
						event INT64 NOT NULL,
						limits JSON,
						days_till_escalation INT64,
						notifications_count INT64 NOT NULL DEFAULT (0),
						created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp)
					) PRIMARY KEY ( user_id, event )`,

					`CREATE TABLE IF NOT EXISTS accounting_rollups (
						node_id BYTES(MAX) NOT NULL,
						start_time TIMESTAMP NOT NULL,
						put_total INT64 NOT NULL,
						get_total INT64 NOT NULL,
						get_audit_total INT64 NOT NULL,
						get_repair_total INT64 NOT NULL,
						put_repair_total INT64 NOT NULL,
						at_rest_total FLOAT64 NOT NULL,
						interval_end_time TIMESTAMP
					) PRIMARY KEY ( node_id, start_time )`,

					`CREATE TABLE IF NOT EXISTS accounting_timestamps (
						name STRING(MAX) NOT NULL,
						value TIMESTAMP NOT NULL
					) PRIMARY KEY ( name )`,

					`CREATE TABLE IF NOT EXISTS billing_balances (
						user_id BYTES(MAX) NOT NULL,
						balance INT64 NOT NULL,
						last_updated TIMESTAMP NOT NULL
					) PRIMARY KEY ( user_id )`,

					`CREATE SEQUENCE IF NOT EXISTS billing_transactions_id OPTIONS (sequence_kind='bit_reversed_positive')`,

					`CREATE TABLE IF NOT EXISTS billing_transactions (
						id INT64 NOT NULL DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE billing_transactions_id)),
						user_id BYTES(MAX) NOT NULL,
						amount INT64 NOT NULL,
						currency STRING(MAX) NOT NULL,
						description STRING(MAX) NOT NULL,
						source STRING(MAX) NOT NULL,
						status STRING(MAX) NOT NULL,
						type STRING(MAX) NOT NULL,
						metadata JSON NOT NULL,
						tx_timestamp TIMESTAMP NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS bucket_bandwidth_rollups (
						bucket_name BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						interval_seconds INT64 NOT NULL,
						action INT64 NOT NULL,
						inline INT64 NOT NULL,
						allocated INT64 NOT NULL,
						settled INT64 NOT NULL
					) PRIMARY KEY ( project_id, bucket_name, interval_start, action )`,

					`CREATE TABLE IF NOT EXISTS bucket_bandwidth_rollup_archives (
						bucket_name BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						interval_seconds INT64 NOT NULL,
						action INT64 NOT NULL,
						inline INT64 NOT NULL,
						allocated INT64 NOT NULL,
						settled INT64 NOT NULL
					) PRIMARY KEY ( bucket_name, project_id, interval_start, action )`,

					`CREATE TABLE IF NOT EXISTS bucket_storage_tallies (
						bucket_name BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						total_bytes INT64 NOT NULL DEFAULT (0),
						inline INT64 NOT NULL,
						remote INT64 NOT NULL,
						total_segments_count INT64 NOT NULL DEFAULT (0),
						remote_segments_count INT64 NOT NULL,
						inline_segments_count INT64 NOT NULL,
						object_count INT64 NOT NULL,
						metadata_size INT64 NOT NULL
					) PRIMARY KEY ( bucket_name, project_id, interval_start )`,

					`CREATE TABLE IF NOT EXISTS coinpayments_transactions (
						id STRING(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						address STRING(MAX) NOT NULL,
						amount_numeric INT64 NOT NULL,
						received_numeric INT64 NOT NULL,
						status INT64 NOT NULL,
						key STRING(MAX) NOT NULL,
						timeout INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS graceful_exit_progress (
						node_id BYTES(MAX) NOT NULL,
						bytes_transferred INT64 NOT NULL,
						pieces_transferred INT64 NOT NULL DEFAULT (0),
						pieces_failed INT64 NOT NULL DEFAULT (0),
						updated_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( node_id )`,

					`CREATE TABLE IF NOT EXISTS graceful_exit_segment_transfer_queue (
						node_id BYTES(MAX) NOT NULL,
						stream_id BYTES(MAX) NOT NULL,
						position INT64 NOT NULL,
						piece_num INT64 NOT NULL,
						root_piece_id BYTES(MAX),
						durability_ratio FLOAT64 NOT NULL,
						queued_at TIMESTAMP NOT NULL,
						requested_at TIMESTAMP,
						last_failed_at TIMESTAMP,
						last_failed_code INT64,
						failed_count INT64,
						finished_at TIMESTAMP,
						order_limit_send_count INT64 NOT NULL DEFAULT (0)
					) PRIMARY KEY ( node_id, stream_id, position, piece_num )`,

					`CREATE TABLE IF NOT EXISTS nodes (
						id BYTES(MAX) NOT NULL,
						address STRING(MAX) NOT NULL DEFAULT (""),
						last_net STRING(MAX) NOT NULL,
						last_ip_port STRING(MAX),
						country_code STRING(MAX),
						protocol INT64 NOT NULL DEFAULT (0),
						email STRING(MAX) NOT NULL,
						wallet STRING(MAX) NOT NULL,
						wallet_features STRING(MAX) NOT NULL DEFAULT (""),
						free_disk INT64 NOT NULL DEFAULT (-1),
						piece_count INT64 NOT NULL DEFAULT (0),
						major INT64 NOT NULL DEFAULT (0),
						minor INT64 NOT NULL DEFAULT (0),
						patch INT64 NOT NULL DEFAULT (0),
						commit_hash STRING(MAX) NOT NULL DEFAULT (""),
						release_timestamp TIMESTAMP NOT NULL DEFAULT ("0001-01-01 00:00:00+00"),
						release BOOL NOT NULL DEFAULT (false),
						latency_90 INT64 NOT NULL DEFAULT (0),
						vetted_at TIMESTAMP,
						created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						updated_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						last_contact_success TIMESTAMP NOT NULL DEFAULT (timestamp_seconds(0)),
						last_contact_failure TIMESTAMP NOT NULL DEFAULT (timestamp_seconds(0)),
						disqualified TIMESTAMP,
						disqualification_reason INT64,
						unknown_audit_suspended TIMESTAMP,
						offline_suspended TIMESTAMP,
						under_review TIMESTAMP,
						exit_initiated_at TIMESTAMP,
						exit_loop_completed_at TIMESTAMP,
						exit_finished_at TIMESTAMP,
						exit_success BOOL NOT NULL DEFAULT (false),
						contained TIMESTAMP,
						last_offline_email TIMESTAMP,
						last_software_update_email TIMESTAMP,
						noise_proto INT64,
						noise_public_key BYTES(MAX),
						debounce_limit INT64 NOT NULL DEFAULT (0),
						features INT64 NOT NULL DEFAULT (0)
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS node_api_versions (
						id BYTES(MAX) NOT NULL,
						api_version INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL,
						updated_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS node_events (
						id BYTES(MAX) NOT NULL,
						email STRING(MAX) NOT NULL,
						last_ip_port STRING(MAX),
						node_id BYTES(MAX) NOT NULL,
						event INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						last_attempted TIMESTAMP,
						email_sent TIMESTAMP
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS node_tags (
						node_id BYTES(MAX) NOT NULL,
						name STRING(MAX) NOT NULL,
						value BYTES(MAX) NOT NULL,
						signed_at TIMESTAMP NOT NULL,
						signer BYTES(MAX) NOT NULL
					) PRIMARY KEY ( node_id, name, signer )`,

					`CREATE TABLE IF NOT EXISTS oauth_clients (
						id BYTES(MAX) NOT NULL,
						encrypted_secret BYTES(MAX) NOT NULL,
						redirect_url STRING(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						app_name STRING(MAX) NOT NULL,
						app_logo_url STRING(MAX) NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS oauth_codes (
						client_id BYTES(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						scope STRING(MAX) NOT NULL,
						redirect_url STRING(MAX) NOT NULL,
						challenge STRING(MAX) NOT NULL,
						challenge_method STRING(MAX) NOT NULL,
						code STRING(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL,
						expires_at TIMESTAMP NOT NULL,
						claimed_at TIMESTAMP
					) PRIMARY KEY ( code )`,

					`CREATE TABLE IF NOT EXISTS oauth_tokens (
						client_id BYTES(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						scope STRING(MAX) NOT NULL,
						kind INT64 NOT NULL,
						token BYTES(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL,
						expires_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( token )`,

					`CREATE TABLE IF NOT EXISTS peer_identities (
						node_id BYTES(MAX) NOT NULL,
						leaf_serial_number BYTES(MAX) NOT NULL,
						chain BYTES(MAX) NOT NULL,
						updated_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( node_id )`,

					`CREATE TABLE IF NOT EXISTS projects (
						id BYTES(MAX) NOT NULL,
						public_id BYTES(MAX),
						name STRING(MAX) NOT NULL,
						description STRING(MAX) NOT NULL,
						usage_limit INT64,
						bandwidth_limit INT64,
						user_specified_usage_limit INT64,
						user_specified_bandwidth_limit INT64,
						segment_limit INT64 DEFAULT (1000000),
						rate_limit INT64,
						burst_limit INT64,
						rate_limit_head INT64,
						burst_limit_head INT64,
						rate_limit_get INT64,
						burst_limit_get INT64,
						rate_limit_put INT64,
						burst_limit_put INT64,
						rate_limit_list INT64,
						burst_limit_list INT64,
						rate_limit_del INT64,
						burst_limit_del INT64,
						max_buckets INT64,
						user_agent BYTES(MAX),
						owner_id BYTES(MAX) NOT NULL,
						salt BYTES(MAX),
						created_at TIMESTAMP NOT NULL,
						default_placement INT64,
						default_versioning INT64 NOT NULL DEFAULT (1),
						prompted_for_versioning_beta BOOL NOT NULL DEFAULT (false),
						passphrase_enc BYTES(MAX),
						passphrase_enc_key_id INT64,
						path_encryption BOOL NOT NULL DEFAULT (true)
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS project_bandwidth_daily_rollups (
						project_id BYTES(MAX) NOT NULL,
						interval_day DATE NOT NULL,
						egress_allocated INT64 NOT NULL,
						egress_settled INT64 NOT NULL,
						egress_dead INT64 NOT NULL DEFAULT (0)
					) PRIMARY KEY ( project_id, interval_day )`,

					`CREATE TABLE IF NOT EXISTS registration_tokens (
						secret BYTES(MAX) NOT NULL,
						owner_id BYTES(MAX),
						project_limit INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( secret )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_registration_tokens_owner_id ON registration_tokens ( owner_id )`,

					`CREATE TABLE IF NOT EXISTS repair_queue (
						stream_id BYTES(MAX) NOT NULL,
						position INT64 NOT NULL,
						attempted_at TIMESTAMP,
						updated_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						segment_health FLOAT64 NOT NULL DEFAULT (1),
						placement INT64
					) PRIMARY KEY ( stream_id, position )`,

					`CREATE TABLE IF NOT EXISTS reputations (
						id BYTES(MAX) NOT NULL,
						audit_success_count INT64 NOT NULL DEFAULT (0),
						total_audit_count INT64 NOT NULL DEFAULT (0),
						vetted_at TIMESTAMP,
						created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						updated_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						disqualified TIMESTAMP,
						disqualification_reason INT64,
						unknown_audit_suspended TIMESTAMP,
						offline_suspended TIMESTAMP,
						under_review TIMESTAMP,
						online_score FLOAT64 NOT NULL DEFAULT (1),
						audit_history BYTES(MAX) NOT NULL,
						audit_reputation_alpha FLOAT64 NOT NULL DEFAULT (1),
						audit_reputation_beta FLOAT64 NOT NULL DEFAULT (0),
						unknown_audit_reputation_alpha FLOAT64 NOT NULL DEFAULT (1),
						unknown_audit_reputation_beta FLOAT64 NOT NULL DEFAULT (0)
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS reset_password_tokens (
						secret BYTES(MAX) NOT NULL,
						owner_id BYTES(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( secret )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_reset_password_tokens_owner_id ON reset_password_tokens ( owner_id )`,

					`CREATE TABLE IF NOT EXISTS reverification_audits (
						node_id BYTES(MAX) NOT NULL,
						stream_id BYTES(MAX) NOT NULL,
						position INT64 NOT NULL,
						piece_num INT64 NOT NULL,
						inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						last_attempt TIMESTAMP,
						reverify_count INT64 NOT NULL DEFAULT (0)
					) PRIMARY KEY ( node_id, stream_id, position )`,

					`CREATE TABLE IF NOT EXISTS revocations (
						revoked BYTES(MAX) NOT NULL,
						api_key_id BYTES(MAX) NOT NULL
					) PRIMARY KEY ( revoked )`,

					`CREATE TABLE IF NOT EXISTS segment_pending_audits (
						node_id BYTES(MAX) NOT NULL,
						stream_id BYTES(MAX) NOT NULL,
						position INT64 NOT NULL,
						piece_id BYTES(MAX) NOT NULL,
						stripe_index INT64 NOT NULL,
						share_size INT64 NOT NULL,
						expected_share_hash BYTES(MAX) NOT NULL,
						reverify_count INT64 NOT NULL
					) PRIMARY KEY ( node_id )`,

					`CREATE TABLE IF NOT EXISTS storagenode_bandwidth_rollups (
						storagenode_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						interval_seconds INT64 NOT NULL,
						action INT64 NOT NULL,
						allocated INT64 DEFAULT (0),
						settled INT64 NOT NULL
					) PRIMARY KEY ( storagenode_id, interval_start, action )`,

					`CREATE TABLE IF NOT EXISTS storagenode_bandwidth_rollup_archives (
						storagenode_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						interval_seconds INT64 NOT NULL,
						action INT64 NOT NULL,
						allocated INT64 DEFAULT (0),
						settled INT64 NOT NULL
					) PRIMARY KEY ( storagenode_id, interval_start, action )`,

					`CREATE TABLE IF NOT EXISTS storagenode_bandwidth_rollups_phase2 (
						storagenode_id BYTES(MAX) NOT NULL,
						interval_start TIMESTAMP NOT NULL,
						interval_seconds INT64 NOT NULL,
						action INT64 NOT NULL,
						allocated INT64 DEFAULT (0),
						settled INT64 NOT NULL
					) PRIMARY KEY ( storagenode_id, interval_start, action )`,

					`CREATE SEQUENCE IF NOT EXISTS storagenode_payments_id OPTIONS (sequence_kind='bit_reversed_positive')`,

					`CREATE TABLE IF NOT EXISTS storagenode_payments (
						id INT64 NOT NULL DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE storagenode_payments_id)),
						created_at TIMESTAMP NOT NULL,
						node_id BYTES(MAX) NOT NULL,
						period STRING(MAX) NOT NULL,
						amount INT64 NOT NULL,
						receipt STRING(MAX),
						notes STRING(MAX)
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS storagenode_paystubs (
						period STRING(MAX) NOT NULL,
						node_id BYTES(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL,
						codes STRING(MAX) NOT NULL,
						usage_at_rest FLOAT64 NOT NULL,
						usage_get INT64 NOT NULL,
						usage_put INT64 NOT NULL,
						usage_get_repair INT64 NOT NULL,
						usage_put_repair INT64 NOT NULL,
						usage_get_audit INT64 NOT NULL,
						comp_at_rest INT64 NOT NULL,
						comp_get INT64 NOT NULL,
						comp_put INT64 NOT NULL,
						comp_get_repair INT64 NOT NULL,
						comp_put_repair INT64 NOT NULL,
						comp_get_audit INT64 NOT NULL,
						surge_percent INT64 NOT NULL,
						held INT64 NOT NULL,
						owed INT64 NOT NULL,
						disposed INT64 NOT NULL,
						paid INT64 NOT NULL,
						distributed INT64 NOT NULL
					) PRIMARY KEY ( period, node_id )`,

					`CREATE TABLE IF NOT EXISTS storagenode_storage_tallies (
						node_id BYTES(MAX) NOT NULL,
						interval_end_time TIMESTAMP NOT NULL,
						data_total FLOAT64 NOT NULL
					) PRIMARY KEY ( interval_end_time, node_id )`,

					`CREATE TABLE IF NOT EXISTS storjscan_payments (
						chain_id INT64 NOT NULL DEFAULT (0),
						block_hash BYTES(MAX) NOT NULL,
						block_number INT64 NOT NULL,
						transaction BYTES(MAX) NOT NULL,
						log_index INT64 NOT NULL,
						from_address BYTES(MAX) NOT NULL,
						to_address BYTES(MAX) NOT NULL,
						token_value INT64 NOT NULL,
						usd_value INT64 NOT NULL,
						status STRING(MAX) NOT NULL,
						block_timestamp TIMESTAMP NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( block_hash, log_index )`,

					`CREATE TABLE IF NOT EXISTS storjscan_wallets (
						user_id BYTES(MAX) NOT NULL,
						wallet_address BYTES(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( user_id, wallet_address )`,

					`CREATE TABLE IF NOT EXISTS stripe_customers (
						user_id BYTES(MAX) NOT NULL,
						customer_id STRING(MAX) NOT NULL,
						billing_customer_id STRING(MAX),
						package_plan STRING(MAX),
						purchased_package_at TIMESTAMP,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( user_id )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_stripe_customers_customer_id ON stripe_customers ( customer_id )`,

					`CREATE TABLE IF NOT EXISTS stripecoinpayments_invoice_project_records (
						id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						storage FLOAT64 NOT NULL,
						egress INT64 NOT NULL,
						objects INT64,
						segments INT64,
						period_start TIMESTAMP NOT NULL,
						period_end TIMESTAMP NOT NULL,
						state INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_stripecoinpayments_invoice_project_records_project_id_period_start_period_end ON stripecoinpayments_invoice_project_records ( project_id, period_start, period_end )`,

					`CREATE TABLE IF NOT EXISTS stripecoinpayments_tx_conversion_rates (
						tx_id STRING(MAX) NOT NULL,
						rate_numeric FLOAT64 NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( tx_id )`,

					`CREATE TABLE IF NOT EXISTS users (
						id BYTES(MAX) NOT NULL,
						external_id STRING(MAX),
						email STRING(MAX) NOT NULL,
						normalized_email STRING(MAX) NOT NULL,
						full_name STRING(MAX) NOT NULL,
						short_name STRING(MAX),
						password_hash BYTES(MAX) NOT NULL,
						new_unverified_email STRING(MAX),
						email_change_verification_step INT64 NOT NULL DEFAULT (0),
						status INT64 NOT NULL,
						status_updated_at TIMESTAMP,
						final_invoice_generated BOOL NOT NULL DEFAULT (false),
						user_agent BYTES(MAX),
						created_at TIMESTAMP NOT NULL,
						project_limit INT64 NOT NULL DEFAULT (0),
						project_bandwidth_limit INT64 NOT NULL DEFAULT (0),
						project_storage_limit INT64 NOT NULL DEFAULT (0),
						project_segment_limit INT64 NOT NULL DEFAULT (0),
						paid_tier BOOL NOT NULL DEFAULT (false),
						position STRING(MAX),
						company_name STRING(MAX),
						company_size INT64,
						working_on STRING(MAX),
						is_professional BOOL NOT NULL DEFAULT (false),
						employee_count STRING(MAX),
						have_sales_contact BOOL NOT NULL DEFAULT (false),
						mfa_enabled BOOL NOT NULL DEFAULT (false),
						mfa_secret_key STRING(MAX),
						mfa_recovery_codes STRING(MAX),
						signup_promo_code STRING(MAX),
						verification_reminders INT64 NOT NULL DEFAULT (0),
						trial_notifications INT64 NOT NULL DEFAULT (0),
						failed_login_count INT64,
						login_lockout_expiration TIMESTAMP,
						signup_captcha FLOAT64,
						default_placement INT64,
						activation_code STRING(MAX),
						signup_id STRING(MAX),
						trial_expiration TIMESTAMP,
						upgrade_time TIMESTAMP
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS user_settings (
						user_id BYTES(MAX) NOT NULL,
						session_minutes INT64,
						passphrase_prompt BOOL,
						onboarding_start BOOL NOT NULL DEFAULT (true),
						onboarding_end BOOL NOT NULL DEFAULT (true),
						onboarding_step STRING(MAX),
						notice_dismissal JSON NOT NULL DEFAULT (JSON "{}")
					) PRIMARY KEY ( user_id )`,

					`CREATE TABLE IF NOT EXISTS value_attributions (
						project_id BYTES(MAX) NOT NULL,
						bucket_name BYTES(MAX) NOT NULL,
						user_agent BYTES(MAX),
						last_updated TIMESTAMP NOT NULL
					) PRIMARY KEY ( project_id, bucket_name )`,

					`CREATE TABLE IF NOT EXISTS verification_audits (
						inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
						stream_id BYTES(MAX) NOT NULL,
						position INT64 NOT NULL,
						expires_at TIMESTAMP,
						encrypted_size INT64 NOT NULL
					) PRIMARY KEY ( inserted_at, stream_id, position )`,

					`CREATE TABLE IF NOT EXISTS webapp_sessions (
						id BYTES(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						ip_address STRING(MAX) NOT NULL,
						user_agent STRING(MAX) NOT NULL,
						status INT64 NOT NULL,
						expires_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( id )`,

					`CREATE TABLE IF NOT EXISTS api_keys (
						id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						head BYTES(MAX) NOT NULL,
						name STRING(MAX) NOT NULL,
						secret BYTES(MAX) NOT NULL,
						user_agent BYTES(MAX),
						created_at TIMESTAMP NOT NULL,
						created_by BYTES(MAX),
						version INT64 NOT NULL DEFAULT (0),
						CONSTRAINT api_keys_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE ,
						CONSTRAINT api_keys_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id)
					) PRIMARY KEY ( id )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_api_keys_head ON api_keys ( head )`,

					`CREATE UNIQUE INDEX IF NOT EXISTS index_api_keys_name_project_id ON api_keys ( name, project_id )`,

					`CREATE TABLE IF NOT EXISTS bucket_metainfos (
						id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						name BYTES(MAX) NOT NULL,
						user_agent BYTES(MAX),
						versioning INT64 NOT NULL DEFAULT (0),
						object_lock_enabled BOOL NOT NULL DEFAULT (false),
						default_retention_mode INT64,
						default_retention_days INT64,
						default_retention_years INT64,
						path_cipher INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL,
						default_segment_size INT64 NOT NULL,
						default_encryption_cipher_suite INT64 NOT NULL,
						default_encryption_block_size INT64 NOT NULL,
						default_redundancy_algorithm INT64 NOT NULL,
						default_redundancy_share_size INT64 NOT NULL,
						default_redundancy_required_shares INT64 NOT NULL,
						default_redundancy_repair_shares INT64 NOT NULL,
						default_redundancy_optimal_shares INT64 NOT NULL,
						default_redundancy_total_shares INT64 NOT NULL,
						placement INT64,
						created_by BYTES(MAX),
						CONSTRAINT bucket_metainfos_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id),
						CONSTRAINT bucket_metainfos_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id)
					) PRIMARY KEY ( project_id, name )`,

					`CREATE TABLE IF NOT EXISTS project_invitations (
						project_id BYTES(MAX) NOT NULL,
						email STRING(MAX) NOT NULL,
						inviter_id BYTES(MAX),
						created_at TIMESTAMP NOT NULL,
						CONSTRAINT project_invitations_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE ,
						CONSTRAINT project_invitations_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES users (id) ON DELETE CASCADE
					) PRIMARY KEY ( project_id, email )`,

					`CREATE TABLE IF NOT EXISTS project_members (
						member_id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						role INT64 NOT NULL DEFAULT (0),
						created_at TIMESTAMP NOT NULL,
						CONSTRAINT project_members_member_id_fkey FOREIGN KEY (member_id) REFERENCES users (id) ON DELETE CASCADE ,
						CONSTRAINT project_members_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
					) PRIMARY KEY ( member_id, project_id )`,

					`CREATE TABLE IF NOT EXISTS stripecoinpayments_apply_balance_intents (
						tx_id STRING(MAX) NOT NULL,
						state INT64 NOT NULL,
						created_at TIMESTAMP NOT NULL,
						CONSTRAINT stripecoinpayments_apply_balance_intents_tx_id_fkey FOREIGN KEY (tx_id) REFERENCES coinpayments_transactions (id) ON DELETE CASCADE
					) PRIMARY KEY ( tx_id )`,

					`CREATE INDEX IF NOT EXISTS accounting_rollups_start_time_index ON accounting_rollups ( start_time )`,

					`CREATE INDEX IF NOT EXISTS billing_transactions_tx_timestamp_index ON billing_transactions ( tx_timestamp )`,

					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_project_id_action_interval_index ON bucket_bandwidth_rollups ( project_id, action, interval_start )`,

					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_action_interval_project_id_index ON bucket_bandwidth_rollups ( action, interval_start, project_id )`,

					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_archive_project_id_action_interval_index ON bucket_bandwidth_rollup_archives ( project_id, action, interval_start )`,

					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_archive_action_interval_project_id_index ON bucket_bandwidth_rollup_archives ( action, interval_start, project_id )`,

					`CREATE INDEX IF NOT EXISTS bucket_storage_tallies_project_id_interval_start_index ON bucket_storage_tallies ( project_id, interval_start )`,

					`CREATE INDEX IF NOT EXISTS bucket_storage_tallies_interval_start_index ON bucket_storage_tallies ( interval_start )`,

					`CREATE INDEX IF NOT EXISTS graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index ON graceful_exit_segment_transfer_queue ( node_id, durability_ratio, queued_at, finished_at, last_failed_at )`,

					`CREATE INDEX IF NOT EXISTS node_last_ip ON nodes ( last_net )`,

					`CREATE INDEX IF NOT EXISTS nodes_dis_unk_off_exit_fin_last_success_index ON nodes ( disqualified, unknown_audit_suspended, offline_suspended, exit_finished_at, last_contact_success )`,

					`CREATE INDEX IF NOT EXISTS nodes_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index ON nodes ( last_contact_success, free_disk, major, minor, patch, vetted_at )`,

					`CREATE INDEX IF NOT EXISTS nodes_dis_unk_aud_exit_init_rel_last_cont_success_stored_index ON nodes ( disqualified, unknown_audit_suspended, exit_initiated_at, release, last_contact_success )`,

					`CREATE INDEX IF NOT EXISTS node_events_email_event_created_at_index ON node_events ( email, event, created_at )`,

					`CREATE INDEX IF NOT EXISTS oauth_clients_user_id_index ON oauth_clients ( user_id )`,

					`CREATE INDEX IF NOT EXISTS oauth_codes_user_id_index ON oauth_codes ( user_id )`,

					`CREATE INDEX IF NOT EXISTS oauth_codes_client_id_index ON oauth_codes ( client_id )`,

					`CREATE INDEX IF NOT EXISTS oauth_tokens_user_id_index ON oauth_tokens ( user_id )`,

					`CREATE INDEX IF NOT EXISTS oauth_tokens_client_id_index ON oauth_tokens ( client_id )`,

					`CREATE INDEX IF NOT EXISTS projects_public_id_index ON projects ( public_id )`,

					`CREATE INDEX IF NOT EXISTS projects_owner_id_index ON projects ( owner_id )`,

					`CREATE INDEX IF NOT EXISTS project_bandwidth_daily_rollup_interval_day_index ON project_bandwidth_daily_rollups ( interval_day )`,

					`CREATE INDEX IF NOT EXISTS repair_queue_updated_at_index ON repair_queue ( updated_at )`,

					`CREATE INDEX IF NOT EXISTS repair_queue_num_healthy_pieces_attempted_at_index ON repair_queue ( segment_health, attempted_at )`,

					`CREATE INDEX IF NOT EXISTS repair_queue_placement_index ON repair_queue ( placement )`,

					`CREATE INDEX IF NOT EXISTS reverification_audits_inserted_at_index ON reverification_audits ( inserted_at )`,

					`CREATE INDEX IF NOT EXISTS storagenode_bandwidth_rollups_interval_start_index ON storagenode_bandwidth_rollups ( interval_start )`,

					`CREATE INDEX IF NOT EXISTS storagenode_bandwidth_rollup_archives_interval_start_index ON storagenode_bandwidth_rollup_archives ( interval_start )`,

					`CREATE INDEX IF NOT EXISTS storagenode_payments_node_id_period_index ON storagenode_payments ( node_id, period )`,

					`CREATE INDEX IF NOT EXISTS storagenode_paystubs_node_id_index ON storagenode_paystubs ( node_id )`,

					`CREATE INDEX IF NOT EXISTS storagenode_storage_tallies_node_id_index ON storagenode_storage_tallies ( node_id )`,

					`CREATE INDEX IF NOT EXISTS storjscan_payments_chain_id_block_number_log_index_index ON storjscan_payments ( chain_id, block_number, log_index )`,

					`CREATE INDEX IF NOT EXISTS storjscan_wallets_wallet_address_index ON storjscan_wallets ( wallet_address )`,

					`CREATE INDEX IF NOT EXISTS stripecoinpayments_invoice_project_records_unbilled_project_id_index ON stripecoinpayments_invoice_project_records ( project_id )`,

					`CREATE INDEX IF NOT EXISTS users_email_status_index ON users ( normalized_email, status )`,

					`CREATE INDEX IF NOT EXISTS trial_expiration_index ON users ( trial_expiration )`,

					`CREATE INDEX IF NOT EXISTS users_external_id_index ON users ( external_id )`,

					`CREATE INDEX IF NOT EXISTS webapp_sessions_user_id_index ON webapp_sessions ( user_id )`,

					`CREATE INDEX IF NOT EXISTS project_invitations_project_id_index ON project_invitations ( project_id )`,

					`CREATE INDEX IF NOT EXISTS project_invitations_email_index ON project_invitations ( email )`,

					`CREATE INDEX IF NOT EXISTS project_members_project_id_index ON project_members ( project_id )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to projects table to track the status of the project",
				Version:     284,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN status INT64 DEFAULT (1)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop all nodes table indexes execept primary key",
				Version:     285,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS nodes_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index`,
					`DROP INDEX IF EXISTS nodes_dis_unk_aud_exit_init_rel_last_cont_success_stored_index`,
					`DROP INDEX IF EXISTS node_last_ip`,
					`DROP INDEX IF EXISTS nodes_dis_unk_off_exit_fin_last_success_index`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add hubspot_object_id column to users",
				Version:     286,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN hubspot_object_id STRING(MAX)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add product_id column to bucket_storage_tallies, bucket_bandwidth_rollups, project_bandwidth_daily_rollups, bucket_bandwidth_rollup_archives",
				Version:     287,
				Action: migrate.SQL{
					`ALTER TABLE bucket_storage_tallies ADD COLUMN product_id INT64`,
					`ALTER TABLE bucket_bandwidth_rollups ADD COLUMN product_id INT64`,
					`ALTER TABLE project_bandwidth_daily_rollups ADD COLUMN product_id INT64`,
					`ALTER TABLE bucket_bandwidth_rollup_archives ADD COLUMN product_id INT64`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add rest api keys table",
				Version:     288,
				Action: migrate.SQL{
					`CREATE TABLE rest_api_keys (
						id BYTES(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						token BYTES(MAX) NOT NULL,
						name STRING(MAX) NOT NULL,
						expires_at TIMESTAMP,
						created_at TIMESTAMP NOT NULL,
						CONSTRAINT rest_api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
					) PRIMARY KEY ( id );`,
					`CREATE UNIQUE INDEX index_rest_api_keys_token ON rest_api_keys ( token );`,
					`CREATE INDEX rest_api_keys_user_id_index ON rest_api_keys ( user_id );`,
					`CREATE INDEX rest_api_keys_name_index ON rest_api_keys ( name );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop table storagenode_bandwidth_rollups_phase2",
				Version:     289,
				Action: migrate.SQL{
					`DROP TABLE IF EXISTS storagenode_bandwidth_rollups_phase2`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to users table to track user type",
				Version:     290,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN kind INT64 NOT NULL DEFAULT (0)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update user kind to 1 (PRO) for paid tier users",
				Version:     291,
				Action: migrate.SQL{
					`UPDATE users SET kind = 1 WHERE paid_tier = true`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add domains table",
				Version:     292,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS domains (
						project_id BYTES(MAX) NOT NULL,
						subdomain STRING(MAX) NOT NULL,
						prefix STRING(MAX) NOT NULL,
						access_id STRING(MAX) NOT NULL,
						created_by BYTES(MAX) NOT NULL,
						created_at TIMESTAMP NOT NULL,
						CONSTRAINT domains_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id),
						CONSTRAINT domains_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id)
					) PRIMARY KEY ( project_id, subdomain )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add placement to value_attributions table",
				Version:     293,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions ADD COLUMN placement INT64`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add tags column to bucket_metainfos table",
				Version:     294,
				Action: migrate.SQL{
					`ALTER TABLE bucket_metainfos ADD COLUMN tags BYTES(MAX);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add api_key_tails table",
				Version:     295,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS api_key_tails (
						tail BYTES(MAX) NOT NULL,
						parent_tail BYTES(MAX) NOT NULL,
						caveat BYTES(MAX) NOT NULL,
						last_used TIMESTAMP NOT NULL
					) PRIMARY KEY ( tail )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update project path encryption to true",
				Version:     296,
				Action: migrate.SQL{
					`UPDATE projects SET path_encryption = true WHERE path_encryption = false`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add root_key_id column to api_key_tails table",
				Version:     297,
				Action: migrate.SQL{
					`ALTER TABLE api_key_tails ADD COLUMN root_key_id BYTES(MAX);`,
					`ALTER TABLE api_key_tails ADD CONSTRAINT api_key_tails_root_key_id_fkey FOREIGN KEY (root_key_id) REFERENCES api_keys (id) ON DELETE CASCADE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop paid_tier column from users table",
				Version:     298,
				Action: migrate.SQL{
					`ALTER TABLE users DROP COLUMN paid_tier;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index to users table on status and status_updated_at",
				Version:     299,
				Action: migrate.SQL{
					`CREATE INDEX users_status_status_updated_at_index ON users ( status, status_updated_at);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column indexed status_updated_at to projects with status",
				Version:     300,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN status_updated_at TIMESTAMP;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index to projects table on status and status_updated_at",
				Version:     301,
				Action: migrate.SQL{
					`CREATE INDEX projects_status_status_updated_at_index ON projects ( status, status_updated_at );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add entitlements table",
				Version:     302,
				Action: migrate.SQL{
					`CREATE TABLE entitlements (
						scope BYTES(MAX) NOT NULL,
						features JSON NOT NULL DEFAULT (JSON "{}"),
						updated_at TIMESTAMP NOT NULL,
						created_at TIMESTAMP NOT NULL
					) PRIMARY KEY ( scope )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add bucket_migrations table",
				Version:     303,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS bucket_migrations (
						id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX) NOT NULL,
						bucket_name BYTES(MAX) NOT NULL,
						from_placement INT64 NOT NULL,
						to_placement INT64 NOT NULL,
						migration_type INT64 NOT NULL,
						state STRING(MAX) NOT NULL,
						bytes_processed INT64 NOT NULL DEFAULT (0),
						error_message STRING(MAX),
						created_at TIMESTAMP NOT NULL,
						updated_at TIMESTAMP NOT NULL,
						completed_at TIMESTAMP,
						CONSTRAINT bucket_migrations_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id)
					) PRIMARY KEY ( id )`,
					`CREATE INDEX bucket_migrations_state_created_at_index ON bucket_migrations ( state, created_at )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add tenant_id column to users",
				Version:     304,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN tenant_id STRING(MAX);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add indexes to users.tenant_id column",
				Version:     305,
				Action: migrate.SQL{
					`CREATE INDEX users_tenant_id_index ON users ( tenant_id );`,
					`CREATE INDEX users_normalized_email_tenant_id_status_index ON users ( normalized_email, tenant_id, status );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "remove unused GE tables",
				Version:     306,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index`,
					`DROP TABLE IF EXISTS graceful_exit_progress`,
					`DROP TABLE IF EXISTS graceful_exit_segment_transfer_queue`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add bucket_eventing_configs table",
				Version:     307,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS bucket_eventing_configs (
						project_id BYTES(MAX) NOT NULL,
						bucket_name BYTES(MAX) NOT NULL,
						config_id STRING(MAX) NOT NULL DEFAULT (GENERATE_UUID()),
						topic_name STRING(MAX) NOT NULL,
						events ARRAY<STRING(128)> NOT NULL,
						filter_prefix BYTES(1024),
						filter_suffix BYTES(1024),
						created_at TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
						updated_at TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp = TRUE),
						CONSTRAINT bucket_eventing_configs_bucket_fkey
							FOREIGN KEY (project_id, bucket_name)
							REFERENCES bucket_metainfos (project_id, name)
							ON DELETE CASCADE
					) PRIMARY KEY ( project_id, bucket_name )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add change_histories table",
				Version:     308,
				Action: migrate.SQL{
					`CREATE TABLE change_histories (
						id BYTES(MAX) NOT NULL,
						admin_email STRING(MAX) NOT NULL,
						user_id BYTES(MAX) NOT NULL,
						project_id BYTES(MAX),
						bucket_name BYTES(MAX),
						item_type STRING(MAX) NOT NULL,
						operation STRING(MAX) NOT NULL,
						reason STRING(MAX) NOT NULL,
						changes JSON NOT NULL,
						timestamp TIMESTAMP NOT NULL DEFAULT (current_timestamp)
					) PRIMARY KEY ( id )`,
					`CREATE INDEX change_history_user_id_timestamp_idx ON change_histories ( user_id, timestamp );`,
					`CREATE INDEX change_history_user_id_item_type_timestamp_idx ON change_histories ( user_id, item_type, timestamp );`,
					`CREATE INDEX change_history_project_id_item_type_timestamp_idx ON change_histories ( project_id, item_type, timestamp );`,
					`CREATE INDEX change_history_bucket_name_timestamp_idx ON change_histories ( bucket_name, timestamp );`,
				},
			},
			// NB: after updating testdata in `testdata`, run
			//     `go generate` to update `migratez.go`.
		},
	}
}

// productionMigrationPostgres returns steps needed for migrating postgres database.
func (db *satelliteDB) productionMigrationPostgres() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Initial setup",
				Version:     103,
				Action: migrate.SQL{
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
					`CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups ( start_time );`,

					`CREATE TABLE accounting_timestamps (
						name text NOT NULL,
						value timestamp with time zone NOT NULL,
						PRIMARY KEY ( name )
					);`,

					`CREATE TABLE bucket_bandwidth_rollups (
						bucket_name bytea NOT NULL,
						interval_start timestamp with time zone NOT NULL,
						interval_seconds integer NOT NULL,
						action integer NOT NULL,
						inline bigint NOT NULL,
						allocated bigint NOT NULL,
						settled bigint NOT NULL,
						project_id bytea NOT NULL ,
						CONSTRAINT bucket_bandwidth_rollups_pk PRIMARY KEY (bucket_name, project_id, interval_start, action)
					);`,
					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_project_id_action_interval_index ON bucket_bandwidth_rollups ( project_id, action, interval_start );`,

					`CREATE TABLE bucket_storage_tallies (
						bucket_name bytea NOT NULL,
						interval_start timestamp with time zone NOT NULL,
						inline bigint NOT NULL,
						remote bigint NOT NULL,
						remote_segments_count integer NOT NULL,
						inline_segments_count integer NOT NULL,
						object_count integer NOT NULL,
						metadata_size bigint NOT NULL,
						project_id bytea NOT NULL,
						CONSTRAINT bucket_storage_tallies_pk PRIMARY KEY (bucket_name, project_id, interval_start)
					);`,

					`CREATE TABLE injuredsegments (
						data bytea NOT NULL,
						attempted timestamp with time zone,
						path bytea NOT NULL,
						num_healthy_pieces integer DEFAULT 52 NOT NULL,
						CONSTRAINT injuredsegments_pk PRIMARY KEY (path)
					);`,
					`CREATE INDEX injuredsegments_attempted_index ON injuredsegments ( attempted );`,
					`CREATE INDEX injuredsegments_num_healthy_pieces_index ON injuredsegments ( num_healthy_pieces );`,

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
						created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						wallet text NOT NULL,
						email text NOT NULL,
						address text NOT NULL DEFAULT '',
						protocol INTEGER NOT NULL DEFAULT 0,
						type INTEGER NOT NULL DEFAULT 0,
						free_disk BIGINT NOT NULL DEFAULT -1,
						latency_90 BIGINT NOT NULL DEFAULT 0,
						last_contact_success TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch',
						last_contact_failure TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'epoch',
						major bigint NOT NULL DEFAULT 0,
						minor bigint NOT NULL DEFAULT 0,
						patch bigint NOT NULL DEFAULT 0,
						hash TEXT NOT NULL DEFAULT '',
						timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT '0001-01-01 00:00:00+00',
						release bool NOT NULL DEFAULT FALSE,
						contained bool NOT NULL DEFAULT FALSE,
						last_net text NOT NULL,
						disqualified timestamp with time zone,
						audit_reputation_alpha double precision NOT NULL DEFAULT 1,
						audit_reputation_beta double precision NOT NULL DEFAULT 0,
						uptime_reputation_alpha double precision NOT NULL DEFAULT 1,
						uptime_reputation_beta double precision NOT NULL DEFAULT 0,
						piece_count bigint NOT NULL DEFAULT 0,
						exit_loop_completed_at timestamp with time zone,
						exit_initiated_at timestamp with time zone,
						exit_finished_at timestamp with time zone,
						exit_success boolean NOT NULL DEFAULT FALSE,
						unknown_audit_reputation_alpha double precision NOT NULL DEFAULT 1,
						unknown_audit_reputation_beta double precision NOT NULL DEFAULT 0,
						suspended timestamp with time zone,
						last_ip_port text,
						vetted_at timestamp with time zone,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX node_last_ip ON nodes ( last_net );`,

					`CREATE TABLE offers (
						id serial NOT NULL,
						name text NOT NULL,
						description text NOT NULL,
						type integer NOT NULL,
						award_credit_duration_days integer,
						invitee_credit_duration_days integer,
						redeemable_cap integer,
						expires_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						status integer NOT NULL,
						award_credit_in_cents integer NOT NULL DEFAULT 0,
						invitee_credit_in_cents integer NOT NULL DEFAULT 0,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE peer_identities (
						node_id bytea NOT NULL,
						leaf_serial_number bytea NOT NULL,
						chain bytea NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( node_id )
					);`,

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

					`CREATE TABLE projects (
						id bytea NOT NULL,
						name text NOT NULL,
						description text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						usage_limit bigint NOT NULL DEFAULT 0,
						partner_id bytea,
						owner_id bytea NOT NULL,
						rate_limit integer,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE registration_tokens (
						secret bytea NOT NULL,
						owner_id bytea UNIQUE,
						project_limit integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( secret )
					);`,

					`CREATE TABLE reset_password_tokens (
						secret bytea NOT NULL,
						owner_id bytea NOT NULL UNIQUE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( secret )
					);`,

					`CREATE TABLE serial_numbers (
						id serial NOT NULL,
						serial_number bytea NOT NULL,
						bucket_id bytea NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX serial_numbers_expires_at_index ON serial_numbers ( expires_at );`,
					`CREATE UNIQUE INDEX serial_number_index ON serial_numbers ( serial_number )`,

					`CREATE TABLE storagenode_bandwidth_rollups (
						storagenode_id bytea NOT NULL,
						interval_start timestamp with time zone NOT NULL,
						interval_seconds integer NOT NULL,
						action integer NOT NULL,
						allocated bigint DEFAULT 0,
						settled bigint NOT NULL,
						PRIMARY KEY ( storagenode_id, interval_start, action )
					);`,

					`CREATE TABLE storagenode_storage_tallies (
						node_id bytea NOT NULL,
						interval_end_time timestamp with time zone NOT NULL,
						data_total double precision NOT NULL,
						CONSTRAINT storagenode_storage_tallies_pkey PRIMARY KEY ( interval_end_time, node_id )
					);`,
					`CREATE INDEX storagenode_storage_tallies_node_id_index ON storagenode_storage_tallies ( node_id );`,

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

					`CREATE TABLE value_attributions (
						bucket_name bytea NOT NULL,
						partner_id bytea NOT NULL,
						last_updated timestamp with time zone NOT NULL,
						project_id bytea NOT NULL,
						PRIMARY KEY (project_id, bucket_name)
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

					`CREATE TABLE project_invoice_stamps (
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						invoice_id bytea NOT NULL UNIQUE,
						start_date timestamp with time zone NOT NULL,
						end_date timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( project_id, start_date, end_date )
					);`,

					`CREATE TABLE project_members (
						member_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( member_id, project_id )
					);`,

					`CREATE TABLE used_serials (
						serial_number_id integer NOT NULL REFERENCES serial_numbers( id ) ON DELETE CASCADE,
						storage_node_id bytea NOT NULL,
						PRIMARY KEY ( serial_number_id, storage_node_id )
					);`,

					`CREATE TABLE user_credits (
						id serial NOT NULL,
						user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						offer_id integer NOT NULL REFERENCES offers( id ),
						referred_by bytea REFERENCES users( id ) ON DELETE SET NULL,
						credits_earned_in_cents integer NOT NULL,
						credits_used_in_cents integer NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						type text NOT NULL,
						PRIMARY KEY ( id ),
						UNIQUE (id, offer_id)
					);`,
					`CREATE UNIQUE INDEX credits_earned_user_id_offer_id ON user_credits (id, offer_id);`,

					`INSERT INTO offers (
						id,
						name,
						description,
						award_credit_in_cents,
						invitee_credit_in_cents,
						expires_at,
						created_at,
						status,
						type,
						award_credit_duration_days,
						invitee_credit_duration_days
					)
					VALUES (
						1,
						'Default referral offer',
						'Is active when no other active referral offer',
						300,
						600,
						'2119-03-14 08:28:24.636949+00',
						'2019-07-14 08:28:24.636949+00',
						1,
						2,
						365,
						14
					),
					(
						2,
						'Default free credit offer',
						'Is active when no active free credit offer',
						0,
						300,
						'2119-03-14 08:28:24.636949+00',
						'2019-07-14 08:28:24.636949+00',
						1,
						1,
						NULL,
						14
					) ON CONFLICT DO NOTHING;`,

					`CREATE TABLE graceful_exit_progress (
						node_id bytea NOT NULL,
						bytes_transferred bigint NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						pieces_transferred bigint NOT NULL DEFAULT 0,
						pieces_failed bigint NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id )
					);`,

					`CREATE TABLE graceful_exit_transfer_queue (
						node_id bytea NOT NULL,
						path bytea NOT NULL,
						piece_num integer NOT NULL,
						durability_ratio double precision NOT NULL,
						queued_at timestamp with time zone NOT NULL,
						requested_at timestamp with time zone,
						last_failed_at timestamp with time zone,
						last_failed_code integer,
						failed_count integer,
						finished_at timestamp with time zone,
						root_piece_id bytea,
						order_limit_send_count integer NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id, path, piece_num )
					);`,

					`CREATE TABLE stripe_customers (
						user_id bytea NOT NULL,
						customer_id text NOT NULL UNIQUE,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( user_id )
					);`,

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
					`CREATE TABLE stripecoinpayments_tx_conversion_rates (
						tx_id text NOT NULL,
						rate bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( tx_id )
					);`,

					`CREATE TABLE coinpayments_transactions (
						id text NOT NULL,
						user_id bytea NOT NULL,
						address text NOT NULL,
						amount bytea NOT NULL,
						received bytea NOT NULL,
						status integer NOT NULL,
						key text NOT NULL,
						timeout integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE stripecoinpayments_apply_balance_intents (
						tx_id text NOT NULL REFERENCES coinpayments_transactions( id ) ON DELETE CASCADE,
						state integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( tx_id )
					);`,

					`CREATE TABLE nodes_offline_times (
						node_id bytea NOT NULL,
						tracked_at timestamp with time zone NOT NULL,
						seconds integer NOT NULL,
						PRIMARY KEY ( node_id, tracked_at )
					);`,
					`CREATE INDEX nodes_offline_times_node_id_index ON nodes_offline_times ( node_id );`,

					`CREATE TABLE coupons (
						id bytea NOT NULL,
						project_id bytea NOT NULL,
						user_id bytea NOT NULL,
						amount bigint NOT NULL,
						description text NOT NULL,
						type integer NOT NULL,
						status integer NOT NULL,
						duration bigint NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,
					`CREATE TABLE coupon_usages (
						coupon_id bytea NOT NULL,
						amount bigint NOT NULL,
						status integer NOT NULL,
						period timestamp with time zone NOT NULL,
						PRIMARY KEY ( coupon_id, period )
					);`,

					`CREATE TABLE reported_serials (
						expires_at timestamp with time zone NOT NULL,
						storage_node_id bytea NOT NULL,
						bucket_id bytea NOT NULL,
						action integer NOT NULL,
						serial_number bytea NOT NULL,
						settled bigint NOT NULL,
						observed_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( expires_at, storage_node_id, bucket_id, action, serial_number )
					);`,

					`CREATE TABLE credits (
						user_id bytea NOT NULL,
						transaction_id text NOT NULL,
						amount bigint NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( transaction_id )
					);`,

					`CREATE TABLE credits_spendings (
						id bytea NOT NULL,
						user_id bytea NOT NULL,
						project_id bytea NOT NULL,
						amount bigint NOT NULL,
						status int NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,

					`CREATE TABLE consumed_serials (
						storage_node_id bytea NOT NULL,
						serial_number bytea NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( storage_node_id, serial_number )
					);`,
					`CREATE INDEX consumed_serials_expires_at_index ON consumed_serials ( expires_at );`,

					`CREATE TABLE pending_serial_queue (
						storage_node_id bytea NOT NULL,
						bucket_id bytea NOT NULL,
						serial_number bytea NOT NULL,
						action integer NOT NULL,
						settled bigint NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( storage_node_id, bucket_id, serial_number )
					);`,

					`CREATE TABLE storagenode_payments (
						id bigserial NOT NULL,
						created_at timestamp with time zone NOT NULL,
						node_id bytea NOT NULL,
						period text NOT NULL,
						amount bigint NOT NULL,
						receipt text,
						notes text,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX storagenode_payments_node_id_period_index ON storagenode_payments ( node_id, period );`,

					`CREATE TABLE storagenode_paystubs (
						period text NOT NULL,
						node_id bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
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
						PRIMARY KEY ( period, node_id )
					);`,
					`CREATE INDEX storagenode_paystubs_node_id_index ON storagenode_paystubs ( node_id );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add missing bucket_bandwidth_rollups_action_interval_project_id_index index",
				Version:     104,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS bucket_bandwidth_rollups_action_interval_project_id_index ON bucket_bandwidth_rollups(action, interval_start, project_id );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Remove all nodes from suspension mode.",
				Version:     105,
				Action: migrate.SQL{
					`UPDATE nodes SET suspended=NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add project_bandwidth_rollup table and populate with current months data",
				Version:     106,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS project_bandwidth_rollups (
						project_id bytea NOT NULL,
						interval_month date NOT NULL,
						egress_allocated bigint NOT NULL,
						PRIMARY KEY ( project_id, interval_month )
					);
					INSERT INTO project_bandwidth_rollups(project_id, interval_month, egress_allocated)  (
						SELECT project_id, date_trunc('MONTH',now())::DATE, sum(allocated)::bigint FROM bucket_bandwidth_rollups
						WHERE action = 2 AND interval_start >= date_trunc('MONTH',now())::timestamp group by project_id)
					ON CONFLICT(project_id, interval_month) DO UPDATE SET egress_allocated = EXCLUDED.egress_allocated::bigint;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add separate bandwidth column",
				Version:     107,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN bandwidth_limit bigint NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "backfill bandwidth column with previous limits",
				Version:     108,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE projects SET bandwidth_limit = usage_limit;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add period column to the credits_spendings table (step 1)",
				Version:     109,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE credits_spendings ADD COLUMN period timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add period column to the credits_spendings table (step 2)",
				Version:     110,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE credits_spendings SET period = 'epoch';`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add period column to the credits_spendings table (step 3)",
				Version:     111,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE credits_spendings ALTER COLUMN period SET NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "fix incorrect calculations on backported paystub data",
				Version:     112,
				Action: migrate.SQL{`
					UPDATE storagenode_paystubs SET
						comp_at_rest = (
							((owed + held - disposed)::float / GREATEST(surge_percent::float / 100, 1))::int
							- comp_get - comp_get_repair - comp_get_audit
						)
					WHERE
						(
							abs(
								((owed + held - disposed)::float / GREATEST(surge_percent::float / 100, 1))::int
								- comp_get - comp_get_repair - comp_get_audit
							) >= 10
							OR comp_at_rest < 0
						)
						AND codes NOT LIKE '%O%'
						AND codes NOT LIKE '%D%'
						AND period < '2020-03'
				`},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop project_id column from coupon table",
				Version:     113,
				Action: migrate.SQL{
					`ALTER TABLE coupons DROP COLUMN project_id;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add new columns for suspension to node tables",
				Version:     114,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN unknown_audit_suspended TIMESTAMP WITH TIME ZONE;`,
					`ALTER TABLE nodes ADD COLUMN offline_suspended TIMESTAMP WITH TIME ZONE;`,
					`ALTER TABLE nodes ADD COLUMN under_review TIMESTAMP WITH TIME ZONE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add revocations database",
				Version:     115,
				Action: migrate.SQL{`
					CREATE TABLE revocations (
						revoked bytea NOT NULL,
						api_key_id bytea NOT NULL,
						PRIMARY KEY ( revoked )
					);
				`},
			},
			{
				DB:          &db.migrationDB,
				Description: "add audit histories database",
				Version:     116,
				Action: migrate.SQL{
					`CREATE TABLE audit_histories (
						node_id bytea NOT NULL,
						history bytea NOT NULL,
						PRIMARY KEY ( node_id )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add node_api_versions table",
				Version:     117,
				Action: migrate.SQL{`
					CREATE TABLE node_api_versions (
						id bytea NOT NULL,
						api_version integer NOT NULL,
						created_at timestamp with time zone NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);
				`},
			},
			{
				DB:          &db.migrationDB,
				Description: "add max_buckets field to projects and an implicit index on bucket_metainfos project_id,name",
				SeparateTx:  true,
				Version:     118,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN max_buckets INTEGER NOT NULL DEFAULT 0;`,
					`ALTER TABLE bucket_metainfos ADD UNIQUE (project_id, name);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add project_limit field to users table",
				Version:     119,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN project_limit INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "back fill user project limits from existing registration tokens",
				Version:     120,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE users SET project_limit = registration_tokens.project_limit FROM registration_tokens WHERE users.id = registration_tokens.owner_id;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop tables related to credits (old deposit bonuses)",
				Version:     121,
				Action: migrate.SQL{
					`DROP TABLE credits;`,
					`DROP TABLE credits_spendings;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop project_invoice_stamps table",
				Version:     122,
				Action: migrate.SQL{
					`DROP TABLE project_invoice_stamps;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop project_invoice_stamps table",
				Version:     123,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN online_score double precision NOT NULL DEFAULT 1;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column and index updated_at to injuredsegments",
				Version:     124,
				Action: migrate.SQL{
					`ALTER TABLE injuredsegments ADD COLUMN updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp;`,
					`CREATE INDEX injuredsegments_updated_at_index ON injuredsegments ( updated_at );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "make limit columns nullable",
				Version:     125,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE projects ALTER COLUMN max_buckets DROP NOT NULL;`,
					`ALTER TABLE projects ALTER COLUMN max_buckets SET DEFAULT 100;`,
					`ALTER TABLE projects ALTER COLUMN usage_limit DROP NOT NULL;`,
					`ALTER TABLE projects ALTER COLUMN usage_limit SET DEFAULT 50000000000;`,
					`ALTER TABLE projects ALTER COLUMN bandwidth_limit DROP NOT NULL;`,
					`ALTER TABLE projects ALTER COLUMN bandwidth_limit SET DEFAULT 50000000000;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "set 0 limits back to default",
				Version:     126,
				Action: migrate.SQL{
					`UPDATE projects SET max_buckets = 100 WHERE max_buckets = 0;`,
					`UPDATE projects SET usage_limit = 50000000000 WHERE usage_limit = 0;`,
					`UPDATE projects SET bandwidth_limit = 50000000000 WHERE bandwidth_limit = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "enable multiple projects for existing users",
				Version:     127,
				Action: migrate.SQL{
					`UPDATE users SET project_limit=0 WHERE project_limit <= 10 AND project_limit > 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop default values for project limits",
				Version:     128,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE projects ALTER COLUMN max_buckets DROP DEFAULT;`,
					`ALTER TABLE projects ALTER COLUMN usage_limit DROP DEFAULT;`,
					`ALTER TABLE projects ALTER COLUMN bandwidth_limit DROP DEFAULT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "reset everyone with default rate limits to NULL",
				Version:     129,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE projects SET max_buckets = NULL WHERE max_buckets <= 100;`,
					`UPDATE projects SET usage_limit = NULL WHERE usage_limit <= 50000000000;`,
					`UPDATE projects SET bandwidth_limit = NULL WHERE bandwidth_limit <= 50000000000;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index for the gracefule exit transfer queue",
				Version:     130,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS graceful_exit_transfer_queue_nid_dr_qa_fa_lfa_index ON graceful_exit_transfer_queue ( node_id, durability_ratio, queued_at, finished_at, last_failed_at );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create table storagenode_bandwidth_rollups_phase2",
				Version:     131,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE TABLE storagenode_bandwidth_rollups_phase2 (
						storagenode_id bytea NOT NULL,
						interval_start timestamp with time zone NOT NULL,
						interval_seconds integer NOT NULL,
						action integer NOT NULL,
						allocated bigint DEFAULT 0,
						settled bigint NOT NULL,
						PRIMARY KEY ( storagenode_id, interval_start, action )
					);`,
				},
			},
			{
				DB: &db.migrationDB,

				Description: "add injuredsegments.segment_health",
				Version:     132,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE injuredsegments ADD COLUMN segment_health double precision NOT NULL DEFAULT 1;`,
					`CREATE INDEX injuredsegments_segment_health_index ON injuredsegments ( segment_health );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Use node_id and start_time for accounting_rollups pkey instead of autogenerated id",
				Version:     133,
				SeparateTx:  true,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error {
					if db.Name() == tagsql.CockroachName {
						_, err := db.ExecContext(ctx,
							`ALTER TABLE accounting_rollups RENAME TO accounting_rollups_original;`,
						)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}

						_, err = db.ExecContext(ctx,
							`CREATE TABLE accounting_rollups (
								node_id bytea NOT NULL,
								start_time timestamp with time zone NOT NULL,
								put_total bigint NOT NULL,
								get_total bigint NOT NULL,
								get_audit_total bigint NOT NULL,
								get_repair_total bigint NOT NULL,
								put_repair_total bigint NOT NULL,
								at_rest_total double precision NOT NULL,
								PRIMARY KEY ( node_id, start_time )
							);
							CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups ( start_time );

							INSERT INTO accounting_rollups (
								node_id, start_time, put_total, get_total, get_audit_total, get_repair_total, put_repair_total, at_rest_total
							)
							SELECT node_id,
								start_time,
								SUM(put_total)::bigint,
								SUM(get_total)::bigint,
								SUM(get_audit_total)::bigint,
								SUM(get_repair_total)::bigint,
								SUM(put_repair_total)::bigint,
								SUM(at_rest_total)
							FROM accounting_rollups_original
							GROUP BY node_id, start_time;

							DROP TABLE accounting_rollups_original;`,
						)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}
						return nil
					}

					_, err := db.ExecContext(ctx,
						`CREATE TABLE accounting_rollups_new (
								node_id bytea NOT NULL,
								start_time timestamp with time zone NOT NULL,
								put_total bigint NOT NULL,
								get_total bigint NOT NULL,
								get_audit_total bigint NOT NULL,
								get_repair_total bigint NOT NULL,
								put_repair_total bigint NOT NULL,
								at_rest_total double precision NOT NULL,
								PRIMARY KEY ( node_id, start_time )
							);
							DROP INDEX accounting_rollups_start_time_index;
							CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups_new ( start_time );

							INSERT INTO accounting_rollups_new (
								node_id, start_time, put_total, get_total, get_audit_total, get_repair_total, put_repair_total, at_rest_total
							)
							SELECT node_id,
								start_time,
								SUM(put_total),
								SUM(get_total),
								SUM(get_audit_total),
								SUM(get_repair_total),
								SUM(put_repair_total),
								SUM(at_rest_total)
							FROM accounting_rollups
							GROUP BY node_id, start_time;

							DROP TABLE accounting_rollups;

							ALTER INDEX accounting_rollups_new_pkey RENAME TO accounting_rollups_pkey;
							ALTER TABLE accounting_rollups_new RENAME TO accounting_rollups;`,
					)
					if err != nil {
						return ErrMigrate.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "drop num_healthy_pieces column from injuredsegments",
				Version:     134,
				Action: migrate.SQL{
					`ALTER TABLE injuredsegments DROP COLUMN num_healthy_pieces;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on storagenode_bandwidth_rollups to improve get nodes since query.",
				Version:     135,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS storagenode_bandwidth_rollups_interval_start_index ON storagenode_bandwidth_rollups ( interval_start );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on bucket_storage_tallies to improve get tallies by project ID.",
				Version:     136,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS bucket_storage_tallies_project_id_index ON bucket_storage_tallies (project_id);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on nodes to improve queries using disqualified, unknown_audit_suspended, exit_finished_at, and last_contact_success.",
				Version:     137,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS nodes_dis_unk_exit_fin_last_success_index ON nodes(disqualified, unknown_audit_suspended, exit_finished_at, last_contact_success);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop node_offline_times table",
				Version:     138,
				SeparateTx:  true,
				Action: migrate.SQL{
					`DROP TABLE nodes_offline_times;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "set default on uptime count columns",
				Version:     139,
				Action: migrate.SQL{
					`ALTER TABLE nodes ALTER COLUMN uptime_success_count SET DEFAULT 0;`,
					`ALTER TABLE nodes ALTER COLUMN total_uptime_count SET DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add distributed column to storagenode_paystubs table",
				Version:     140,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error {
					_, err := db.ExecContext(ctx, `
							ALTER TABLE storagenode_paystubs ADD COLUMN distributed BIGINT;
						`)
					if err != nil {
						return ErrMigrate.Wrap(err)
					}

					_, err = db.ExecContext(ctx, `
							UPDATE storagenode_paystubs ps
							SET distributed = coalesce((
								SELECT sum(amount)::bigint
								FROM storagenode_payments pm
								WHERE pm.period = ps.period
								  AND pm.node_id = ps.node_id
							), 0);
						`)
					if err != nil {
						return ErrMigrate.Wrap(err)
					}

					_, err = db.ExecContext(ctx, `
							ALTER TABLE storagenode_paystubs ALTER COLUMN distributed SET NOT NULL;
						`)
					if err != nil {
						return ErrMigrate.Wrap(err)
					}

					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns for professional users",
				Version:     141,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN position text;`,
					`ALTER TABLE users ADD COLUMN company_name text;`,
					`ALTER TABLE users ADD COLUMN working_on text;`,
					`ALTER TABLE users ADD COLUMN company_size int;`,
					`ALTER TABLE users ADD COLUMN is_professional boolean NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop the obsolete (name, project_id) index from bucket_metainfos table.",
				Version:     142,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, db tagsql.DB, tx tagsql.Tx) error {
					if db.Name() == tagsql.CockroachName {
						_, err := db.ExecContext(ctx,
							`DROP INDEX bucket_metainfos_name_project_id_key CASCADE;`,
						)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}
						return nil
					}

					_, err := db.ExecContext(ctx,
						`ALTER TABLE bucket_metainfos DROP CONSTRAINT bucket_metainfos_name_project_id_key;`,
					)
					if err != nil {
						return ErrMigrate.Wrap(err)
					}
					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "add storagenode_bandwidth_rollups_archives and bucket_bandwidth_rollup_archives",
				Version:     143,
				SeparateTx:  true,
				Action: migrate.SQL{
					`
                    CREATE TABLE storagenode_bandwidth_rollup_archives (
                        storagenode_id bytea NOT NULL,
                        interval_start timestamp with time zone NOT NULL,
                        interval_seconds integer NOT NULL,
                        action integer NOT NULL,
                        allocated bigint DEFAULT 0,
                        settled bigint NOT NULL,
                        PRIMARY KEY ( storagenode_id, interval_start, action )
                    );`,
					`CREATE TABLE bucket_bandwidth_rollup_archives (
                        bucket_name bytea NOT NULL,
                        project_id bytea NOT NULL,
                        interval_start timestamp with time zone NOT NULL,
                        interval_seconds integer NOT NULL,
                        action integer NOT NULL,
                        inline bigint NOT NULL,
                        allocated bigint NOT NULL,
                        settled bigint NOT NULL,
                        PRIMARY KEY ( bucket_name, project_id, interval_start, action )
                    );`,
					`CREATE INDEX bucket_bandwidth_rollups_archive_project_id_action_interval_index ON bucket_bandwidth_rollup_archives ( project_id, action, interval_start );`,
					`CREATE INDEX bucket_bandwidth_rollups_archive_action_interval_project_id_index ON bucket_bandwidth_rollup_archives ( action, interval_start, project_id );`,
					`CREATE INDEX storagenode_bandwidth_rollup_archives_interval_start_index ON storagenode_bandwidth_rollup_archives (interval_start);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "delete deprecated and unused serial tables",
				Version:     144,
				Action: migrate.SQL{
					`DROP TABLE used_serials;`,
					`DROP TABLE reported_serials;`,
					`DROP TABLE pending_serial_queue;`,
					`DROP TABLE serial_numbers;`,
					`DROP TABLE consumed_serials;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on bucket_storage_tallies to improve get tallies by project ID and bucket name lookups by time interval. This replaces bucket_storage_tallies_project_id_index.",
				Version:     145,
				SeparateTx:  true,
				Action: migrate.SQL{
					`
					CREATE INDEX IF NOT EXISTS bucket_storage_tallies_project_id_interval_start_index ON bucket_storage_tallies ( project_id, interval_start );
					DROP INDEX IF EXISTS bucket_storage_tallies_project_id_index;
					`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "nodes add wallet_features column",
				Version:     146,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN wallet_features text NOT NULL DEFAULT '';`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add employee_count column on users",
				Version:     147,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN employee_count text;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "empty migration to fix backwards compat test discrepancy with release tag",
				Version:     148,
				Action:      migrate.SQL{},
			},
			{
				DB:          &db.migrationDB,
				Description: "add coupon_codes table and add nullable coupon_code_name to coupons table",
				Version:     149,
				Action: migrate.SQL{
					`CREATE TABLE coupon_codes (
						id bytea NOT NULL,
						name text NOT NULL,
						amount bigint NOT NULL,
						description text NOT NULL,
						type integer NOT NULL,
						duration bigint NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id ),
						UNIQUE ( name )
					);`,
					`ALTER TABLE coupons ADD COLUMN coupon_code_name text;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop columns uptime_reputation_alpha and uptime_reputation_beta",
				Version:     150,
				Action: migrate.SQL{
					`ALTER TABLE nodes DROP COLUMN uptime_reputation_alpha;`,
					`ALTER TABLE nodes DROP COLUMN uptime_reputation_beta;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "set default project and usage limits on existing users",
				Version:     151,
				Action: migrate.SQL{
					`UPDATE users SET project_limit = 10 WHERE project_limit = 0;`,
					// 500 GB = 5e11 bytes
					`UPDATE projects SET usage_limit = 500000000000 WHERE usage_limit IS NULL;`,
					`UPDATE projects SET bandwidth_limit = 500000000000 WHERE bandwidth_limit IS NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add offline_suspended to index on nodes",
				Version:     152,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS nodes_dis_unk_off_exit_fin_last_success_index ON nodes (disqualified, unknown_audit_suspended, offline_suspended, exit_finished_at, last_contact_success);`,
					`DROP INDEX IF EXISTS nodes_dis_unk_exit_fin_last_success_index;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add nullable coupons.duration_new, migrate coupon_codes.duration to be nullable",
				Version:     153,
				Action: migrate.SQL{
					`ALTER TABLE coupons ADD COLUMN billing_periods bigint;`,
					`ALTER TABLE coupon_codes ADD COLUMN billing_periods bigint;`,
					`ALTER TABLE coupon_codes DROP COLUMN duration;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "move duration over to billing_periods",
				Version:     154,
				Action: migrate.SQL{
					`UPDATE coupons SET billing_periods = duration;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add have_sales_contact column on users",
				Version:     155,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN have_sales_contact boolean NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create indexes used in production",
				Version:     156,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					storingClause := func(fields ...string) string {
						if db.impl == dbutil.Cockroach {
							return fmt.Sprintf("STORING (%s)", strings.Join(fields, ", "))
						}

						return ""
					}
					indexes := [3]string{
						`CREATE INDEX IF NOT EXISTS injuredsegments_num_healthy_pieces_attempted_index
							ON injuredsegments (segment_health, attempted NULLS FIRST)`,
						`CREATE INDEX IF NOT EXISTS  nodes_type_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index
							ON nodes (type, last_contact_success, free_disk, major, minor, patch, vetted_at)
							` + storingClause("last_net", "address", "last_ip_port") + `
							WHERE disqualified IS NULL AND
							unknown_audit_suspended IS NULL AND
							exit_initiated_at IS NULL AND
							release = true AND
							last_net != ''`,
						`CREATE INDEX IF NOT EXISTS  nodes_dis_unk_aud_exit_init_rel_type_last_cont_success_stored_index
							ON nodes (disqualified ASC, unknown_audit_suspended ASC, exit_initiated_at ASC, release ASC, type ASC, last_contact_success DESC)
							` + storingClause("free_disk", "minor", "major", "patch", "vetted_at", "last_net", "address", "last_ip_port") + `
							WHERE disqualified IS NULL AND
							unknown_audit_suspended IS NULL AND
							exit_initiated_at IS NULL AND
							release = true`,
					}

					for _, s := range indexes {
						_, err := tx.ExecContext(ctx, s)
						if err != nil {
							return err
						}
					}

					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "drop unused columns total_uptime_count and uptime_success_count on nodes table",
				Version:     157,
				Action: migrate.SQL{
					`ALTER TABLE nodes DROP COLUMN total_uptime_count;`,
					`ALTER TABLE nodes DROP COLUMN uptime_success_count;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new table for computing project bandwidth daily usage.",
				Version:     158,
				Action: migrate.SQL{
					`CREATE TABLE project_bandwidth_daily_rollups (
						project_id bytea NOT NULL,
						interval_day date NOT NULL,
						egress_allocated bigint NOT NULL,
						egress_settled bigint NOT NULL,
						PRIMARY KEY ( project_id, interval_day )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "migrate non-expiring coupons to expire in 2 billing periods",
				Version:     159,
				Action: migrate.SQL{
					`UPDATE coupons SET billing_periods = 2 WHERE billing_periods is NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to track dead allocated bandwidth",
				Version:     160,
				Action: migrate.SQL{
					`ALTER TABLE project_bandwidth_daily_rollups ADD COLUMN egress_dead bigint NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add table for node reputation",
				Version:     161,
				Action: migrate.SQL{
					`CREATE TABLE reputations (
						id bytea NOT NULL,
						audit_success_count bigint NOT NULL DEFAULT 0,
						total_audit_count bigint NOT NULL DEFAULT 0,
						vetted_at timestamp with time zone,
						created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						contained boolean NOT NULL DEFAULT false,
						disqualified timestamp with time zone,
						suspended timestamp with time zone,
						unknown_audit_suspended timestamp with time zone,
						offline_suspended timestamp with time zone,
						under_review timestamp with time zone,
						online_score double precision NOT NULL DEFAULT 1,
						audit_history bytea NOT NULL,
						audit_reputation_alpha double precision NOT NULL DEFAULT 1,
						audit_reputation_beta double precision NOT NULL DEFAULT 0,
						unknown_audit_reputation_alpha double precision NOT NULL DEFAULT 1,
						unknown_audit_reputation_beta double precision NOT NULL DEFAULT 0,
						PRIMARY KEY ( id )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add stream_id and position columns to graceful_exit_transfer_queue",
				Version:     162,
				Action: migrate.SQL{
					`CREATE TABLE graceful_exit_segment_transfer_queue (
						node_id bytea NOT NULL,
						stream_id bytea NOT NULL,
						position bigint NOT NULL,
						piece_num integer NOT NULL,
						root_piece_id bytea,
						durability_ratio double precision NOT NULL,
						queued_at timestamp with time zone NOT NULL,
						requested_at timestamp with time zone,
						last_failed_at timestamp with time zone,
						last_failed_code integer,
						failed_count integer,
						finished_at timestamp with time zone,
						order_limit_send_count integer NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id, stream_id, position, piece_num )
					);`,
					`CREATE INDEX graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index ON graceful_exit_segment_transfer_queue ( node_id, durability_ratio, queued_at, finished_at, last_failed_at ) ;`,
					`ALTER TABLE graceful_exit_progress
						ADD COLUMN uses_segment_transfer_queue boolean NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create segment_pending_audits table, replacement for pending_audits",
				Version:     163,
				Action: migrate.SQL{
					`CREATE TABLE segment_pending_audits (
						node_id bytea NOT NULL,
						stream_id bytea NOT NULL,
						position bigint NOT NULL,
						piece_id bytea NOT NULL,
						stripe_index bigint NOT NULL,
						share_size bigint NOT NULL,
						expected_share_hash bytea NOT NULL,
						reverify_count bigint NOT NULL,
						PRIMARY KEY ( node_id )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add paid_tier column to users table",
				Version:     164,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN paid_tier bool NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add repair_queue table, replacement for injuredsegments table",
				Version:     165,
				Action: migrate.SQL{
					`CREATE TABLE repair_queue (
						stream_id bytea NOT NULL,
						position bigint NOT NULL,
						attempted_at timestamp with time zone,
						updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						segment_health double precision NOT NULL DEFAULT 1,
						PRIMARY KEY ( stream_id, position )
					)`,
					`CREATE INDEX repair_queue_updated_at_index ON repair_queue ( updated_at )`,
					`CREATE INDEX repair_queue_num_healthy_pieces_attempted_at_index ON repair_queue ( segment_health, attempted_at NULLS FIRST)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add total_bytes table and total_segments_count for bucket_storage_tallies table",
				Version:     166,
				Action: migrate.SQL{
					`ALTER TABLE bucket_storage_tallies ADD COLUMN total_bytes bigint NOT NULL DEFAULT 0;`,
					`ALTER TABLE bucket_storage_tallies ADD COLUMN total_segments_count integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add multi-factor authentication columns mfa_enabled, mfa_secret_key, and mfa_recovery_codes into users table",
				Version:     167,
				Action: migrate.SQL{
					`ALTER TABLE users
						ADD COLUMN mfa_enabled boolean NOT NULL DEFAULT false,
						ADD COLUMN mfa_secret_key text,
						ADD COLUMN mfa_recovery_codes text;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "migrate audit score related data from overlaycache into reputations table",
				Version:     168,
				Action: migrate.SQL{
					`TRUNCATE TABLE reputations;`,
					`INSERT INTO reputations (
						id,
						audit_success_count,
						total_audit_count,
						vetted_at,
						created_at,
						updated_at,
						contained,
						disqualified,
						suspended,
						unknown_audit_suspended,
						offline_suspended,
						under_review,
						online_score,
						audit_history,
						audit_reputation_alpha,
						audit_reputation_beta,
						unknown_audit_reputation_alpha,
						unknown_audit_reputation_beta
						)
						SELECT
							n.id,
							n.audit_success_count,
							n.total_audit_count,
							n.vetted_at,
							n.created_at,
							n.updated_at,
							n.contained,
							n.disqualified,
							n.suspended,
							n.unknown_audit_suspended,
							n.offline_suspended,
							n.under_review,
							n.online_score,
							audit_histories.history,
							n.audit_reputation_alpha,
							n.audit_reputation_beta,
							n.unknown_audit_reputation_alpha,
							n.unknown_audit_reputation_beta
							FROM nodes as n INNER JOIN audit_histories ON n.id = audit_histories.node_id;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop tables after metaloop refactoring",
				Version:     169,
				Action: migrate.SQL{
					`DROP TABLE pending_audits`,
					`DROP TABLE irreparabledbs`,
					`DROP TABLE injuredsegments`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop audit_history table",
				Version:     170,
				Action: migrate.SQL{
					`DROP TABLE audit_histories`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop audit and unknown audit reputation alpha and beta from nodes table",
				Version:     171,
				Action: migrate.SQL{
					`ALTER TABLE nodes DROP COLUMN audit_reputation_alpha;`,
					`ALTER TABLE nodes DROP COLUMN audit_reputation_beta;`,
					`ALTER TABLE nodes DROP COLUMN unknown_audit_reputation_alpha;`,
					`ALTER TABLE nodes DROP COLUMN unknown_audit_reputation_beta;`,
					`ALTER TABLE nodes DROP COLUMN audit_success_count`,
					`ALTER TABLE nodes DROP COLUMN online_score`,
					`ALTER TABLE nodes DROP COLUMN total_audit_count`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add burst_limit to projects table",
				Version:     172,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN burst_limit int;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop graceful_exit_transfer_queue table",
				Version:     173,
				Action: migrate.SQL{
					`DROP TABLE graceful_exit_transfer_queue`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add user_agent bytes to the value_attributions, users, projects, api_keys and bucket_metainfos tables",
				Version:     174,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions ADD COLUMN user_agent bytea;`,
					`ALTER TABLE users ADD COLUMN user_agent bytea;`,
					`ALTER TABLE projects ADD COLUMN user_agent bytea;`,
					`ALTER TABLE api_keys ADD COLUMN user_agent bytea;`,
					`ALTER TABLE bucket_metainfos ADD COLUMN user_agent bytea;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop column uses_segment_transfer_queue from graceful_exit_progress",
				Version:     175,
				Action: migrate.SQL{
					`ALTER TABLE graceful_exit_progress DROP COLUMN uses_segment_transfer_queue;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add signup_promo_code column on users",
				Version:     176,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN signup_promo_code text;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column segments to invoice_project_records table and drop NOT NULL constraint for objects column",
				Version:     177,
				Action: migrate.SQL{
					`ALTER TABLE stripecoinpayments_invoice_project_records ADD COLUMN segments bigint;`,
					`ALTER TABLE stripecoinpayments_invoice_project_records ALTER COLUMN objects DROP NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add placement to bucket_metainfos and country_code to nodes (geofencing) ",
				Version:     178,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN country_code text;`,
					`ALTER TABLE bucket_metainfos ADD COLUMN placement integer;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add disqualification_reason to nodes",
				Version:     179,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN disqualification_reason integer`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add project_bandwidth_limit and project_storage_limit to the user table",
				Version:     180,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN project_bandwidth_limit bigint NOT NULL DEFAULT 0;`,
					`ALTER TABLE users ADD COLUMN project_storage_limit bigint NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add project_bandwidth_limit and project_storage_limit to the user table",
				Version:     181,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE users SET project_bandwidth_limit = 50000000000, project_storage_limit = 50000000000
                                         WHERE (project_bandwidth_limit = 0 AND project_storage_limit = 0 AND paid_tier = false);`,
					`UPDATE users SET project_bandwidth_limit = 100000000000000, project_storage_limit = 25000000000000
                                         WHERE (project_bandwidth_limit = 0 AND project_storage_limit = 0 AND paid_tier = true);`,
					`UPDATE users SET project_limit = 3 WHERE project_limit = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add segment_limit to the projects table",
				Version:     182,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN segment_limit bigint DEFAULT 1000000`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add project_segment_limit to the users table",
				Version:     183,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN project_segment_limit bigint NOT NULL DEFAULT 0`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add last_verification_reminder to the users table",
				Version:     184,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN last_verification_reminder timestamp with time zone`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add oauth_clients table and user index",
				Version:     185,
				Action: migrate.SQL{
					`CREATE TABLE oauth_clients (
						id bytea NOT NULL,
						encrypted_secret bytea NOT NULL,
						redirect_url text NOT NULL,
						user_id bytea NOT NULL,
						app_name text NOT NULL,
						app_logo_url text NOT NULL,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX oauth_clients_user_id_index ON oauth_clients ( user_id ) ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add oauth_codes and oauth_tokens table",
				Version:     186,
				Action: migrate.SQL{
					`CREATE TABLE oauth_codes (
						client_id bytea NOT NULL,
						user_id bytea NOT NULL,
						scope text NOT NULL,
						redirect_url text NOT NULL,
						challenge text NOT NULL,
						challenge_method text NOT NULL,
						code text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						claimed_at timestamp with time zone,
						PRIMARY KEY ( code )
					);`,
					`CREATE INDEX oauth_codes_user_id_index ON oauth_codes ( user_id ) ;`,
					`CREATE INDEX oauth_codes_client_id_index ON oauth_codes ( client_id ) ;`,
					`CREATE TABLE oauth_tokens (
						client_id bytea NOT NULL,
						user_id bytea NOT NULL,
						scope text NOT NULL,
						kind integer NOT NULL,
						token bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( token )
					)`,
					`CREATE INDEX oauth_tokens_user_id_index ON oauth_tokens ( user_id ) ;`,
					`CREATE INDEX oauth_tokens_client_id_index ON oauth_tokens ( client_id ) ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop contained from nodes and reputations",
				Version:     187,
				Action: migrate.SQL{
					`ALTER TABLE nodes DROP COLUMN contained;`,
					`ALTER TABLE reputations DROP COLUMN contained;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "migrate users/projects to correct segment limit",
				Version:     188,
				Action: migrate.SQL{
					`UPDATE users SET
						project_segment_limit = CASE WHEN paid_tier = true THEN 1000000 ELSE 150000 END;`,
					`UPDATE projects SET segment_limit = 150000
						WHERE owner_id NOT IN (SELECT id FROM users WHERE paid_tier = true);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns to coinpayments tables to replace gob-encoded big.Floats",
				Version:     189,
				Action: migrate.SQL{
					`ALTER TABLE coinpayments_transactions ALTER COLUMN amount DROP NOT NULL;`,
					`ALTER TABLE coinpayments_transactions ALTER COLUMN received DROP NOT NULL;`,
					`ALTER TABLE coinpayments_transactions RENAME COLUMN amount TO amount_gob;`,
					`ALTER TABLE coinpayments_transactions RENAME COLUMN received TO received_gob;`,
					`ALTER TABLE coinpayments_transactions ADD COLUMN amount_numeric int8;`,
					`ALTER TABLE coinpayments_transactions ADD COLUMN received_numeric int8;`,
					`ALTER TABLE stripecoinpayments_tx_conversion_rates ALTER COLUMN rate DROP NOT NULL;`,
					`ALTER TABLE stripecoinpayments_tx_conversion_rates RENAME COLUMN rate TO rate_gob;`,
					`ALTER TABLE stripecoinpayments_tx_conversion_rates ADD COLUMN rate_numeric double precision;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "change segment limit default value to 100M for users from paid tier",
				Version:     190,
				Action: migrate.SQL{
					`UPDATE users SET project_segment_limit = 100000000 WHERE paid_tier = true`,
					`UPDATE projects SET segment_limit = 100000000
						WHERE owner_id IN (SELECT id FROM users WHERE paid_tier = true);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "make _numeric fields not null (all are now populated)",
				Version:     191,
				Action: migrate.SQL{
					`ALTER TABLE coinpayments_transactions ALTER COLUMN amount_numeric SET NOT NULL;`,
					`ALTER TABLE coinpayments_transactions ALTER COLUMN received_numeric SET NOT NULL;`,
					`ALTER TABLE stripecoinpayments_tx_conversion_rates ALTER COLUMN rate_numeric SET NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns to users table to control failed login attempts (disallow brute forcing)",
				Version:     192,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN failed_login_count integer;`,
					`ALTER TABLE users ADD COLUMN login_lockout_expiration timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "make zero project related columns to have default values",
				Version:     193,
				Action: migrate.SQL{
					`UPDATE users SET
						project_bandwidth_limit = 150000000000,
						project_storage_limit = 150000000000,
						project_segment_limit = 150000,
						project_limit = 1
					WHERE (
						project_bandwidth_limit = 0 AND
						project_storage_limit = 0 AND
						project_limit = 0 AND
						paid_tier = false
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop suspended column on reputations and nodes",
				Version:     194,
				Action: migrate.SQL{
					`ALTER TABLE reputations DROP COLUMN suspended;`,
					`ALTER TABLE nodes DROP COLUMN suspended;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create webapp_sessions table",
				Version:     195,
				Action: migrate.SQL{
					`CREATE TABLE webapp_sessions (
						id bytea NOT NULL,
						user_id bytea NOT NULL,
						ip_address text NOT NULL,
						user_agent text NOT NULL,
						status integer NOT NULL,
						expires_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX webapp_sessions_user_id_index ON webapp_sessions ( user_id ) ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add verification_reminders column to users",
				Version:     196,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN verification_reminders INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add disqualification_reason to reputations",
				Version:     197,
				Action: migrate.SQL{
					`ALTER TABLE reputations ADD COLUMN disqualification_reason integer`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add storjscan_wallets",
				Version:     198,
				Action: migrate.SQL{
					`CREATE TABLE storjscan_wallets (
						user_id bytea NOT NULL,
						wallet_address bytea NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( user_id, wallet_address )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add billing_transactions",
				Version:     199,
				Action: migrate.SQL{
					`CREATE TABLE billing_transactions (
						tx_id bytea NOT NULL,
						user_id bytea NOT NULL,
						amount bigint NOT NULL,
						currency text NOT NULL,
						description text NOT NULL,
						type integer NOT NULL,
						timestamp timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( tx_id )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add storjscan_payments table and index on block number and log index",
				Version:     200,
				Action: migrate.SQL{
					`CREATE TABLE storjscan_payments (
						 block_hash bytea NOT NULL,
						 block_number bigint NOT NULL,
						 transaction bytea NOT NULL,
						 log_index integer NOT NULL,
						 from_address bytea NOT NULL,
						 to_address bytea NOT NULL,
						 token_value bigint NOT NULL,
						 usd_value bigint NOT NULL,
						 status text NOT NULL,
						 timestamp timestamp with time zone NOT NULL,
						 created_at timestamp with time zone NOT NULL,
						 PRIMARY KEY ( block_hash, log_index )
					); `,
					`CREATE INDEX storjscan_payments_block_number_log_index_index ON storjscan_payments ( block_number, log_index );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add projects.public_id",
				Version:     201,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN public_id bytea;`,
					`CREATE INDEX IF NOT EXISTS projects_public_id_index ON projects ( public_id );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add accounting_rollups.interval_end_time column",
				Version:     202,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE accounting_rollups ADD COLUMN interval_end_time TIMESTAMP WITH TIME ZONE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Backfill accounting_rollups.interval_end_time with start_time",
				Version:     203,
				SeparateTx:  true,
				Action: migrate.SQL{
					`UPDATE accounting_rollups SET interval_end_time = start_time WHERE interval_end_time = NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop billing table to change primary key.",
				Version:     204,
				Action: migrate.SQL{
					`DROP TABLE billing_transactions;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add billing tables for user balances and transactions",
				Version:     205,
				Action: migrate.SQL{
					`CREATE TABLE billing_balances (
						user_id bytea NOT NULL,
						balance bigint NOT NULL,
						last_updated timestamp with time zone NOT NULL,
						PRIMARY KEY ( user_id )
                    ); `,
					`CREATE TABLE billing_transactions (
						id bigserial NOT NULL,
						user_id bytea NOT NULL,
						amount bigint NOT NULL,
						currency text NOT NULL,
						description text NOT NULL,
						source text NOT NULL,
						status text NOT NULL,
						type text NOT NULL,
						metadata jsonb NOT NULL,
						timestamp timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id )
					); `,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add projects.salt",
				Version:     206,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN salt bytea;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on wallet address to improve queries.",
				Version:     207,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS storjscan_wallets_wallet_address_index ON storjscan_wallets ( wallet_address );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on billing transaction timestamp to improve queries.",
				Version:     208,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS billing_transactions_timestamp_index ON billing_transactions ( timestamp );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "reset all non-DQ'd node audit reputations for new system",
				Version:     209,
				Action: migrate.SQL{
					`UPDATE reputations SET audit_reputation_alpha = 1000, audit_reputation_beta = 0
						WHERE disqualified IS NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add signup_captcha column to users table",
				Version:     210,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN signup_captcha double precision;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Drop now-unused gob-encoded columns",
				Version:     211,
				Action: migrate.SQL{
					`ALTER TABLE coinpayments_transactions DROP COLUMN amount_gob, DROP COLUMN received_gob;`,
					`ALTER TABLE stripecoinpayments_tx_conversion_rates DROP COLUMN rate_gob;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add user_specified_usage_limit and user_specified_bandwidth_limit columns",
				Version:     212,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN user_specified_usage_limit bigint;`,
					`ALTER TABLE projects ADD COLUMN user_specified_bandwidth_limit bigint;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Create table for pending reverification audits",
				Version:     213,
				Action: migrate.SQL{
					`CREATE TABLE reverification_audits (
						node_id bytea NOT NULL,
						stream_id bytea NOT NULL,
						position bigint NOT NULL,
						piece_num integer NOT NULL,
						inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						last_attempt timestamp with time zone,
						reverify_count bigint NOT NULL DEFAULT 0,
						PRIMARY KEY ( node_id, stream_id, position )
    				);`,
					`CREATE INDEX IF NOT EXISTS reverification_audits_inserted_at_index ON reverification_audits ( inserted_at );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Create table for node events",
				Version:     214,
				Action: migrate.SQL{
					`CREATE TABLE node_events (
						id bytea NOT NULL,
						node_id bytea NOT NULL,
						email text NOT NULL,
						event integer NOT NULL,
						created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						email_sent timestamp with time zone,
						PRIMARY KEY ( id )
					);`,
					`CREATE INDEX IF NOT EXISTS node_events_email_event_created_at_index ON node_events ( email, event, created_at )
						WHERE email_sent IS NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Create table for verification queue",
				Version:     215,
				Action: migrate.SQL{
					`CREATE TABLE verification_audits (
					    inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
					    stream_id bytea NOT NULL,
					    position bigint NOT NULL,
					    expires_at timestamp with time zone,
					    encrypted_size integer NOT NULL,
					    PRIMARY KEY ( inserted_at, stream_id, position )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add column contained to nodes table",
				Version:     216,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN contained timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add columns last_offline_email and last_software_update_email",
				Version:     217,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN last_offline_email timestamp with time zone;`,
					`ALTER TABLE nodes ADD COLUMN last_software_update_email timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add column last_attempted to node_events",
				Version:     218,
				Action: migrate.SQL{
					`ALTER TABLE node_events ADD COLUMN last_attempted timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Create account_freeze_events table",
				Version:     219,
				Action: migrate.SQL{
					`CREATE TABLE account_freeze_events (
						user_id bytea NOT NULL,
						event integer NOT NULL,
						limits jsonb,
						created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
						PRIMARY KEY ( user_id, event )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop tables related to coupons, offers, and credits",
				Version:     220,
				Action: migrate.SQL{
					`DROP TABLE user_credits;`,
					`DROP TABLE coupon_usages;`,
					`DROP TABLE coupon_codes;`,
					`DROP TABLE coupons;`,
					`DROP TABLE offers;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop project_bandwidth_rollups table",
				Version:     221,
				Action: migrate.SQL{
					`DROP TABLE project_bandwidth_rollups`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add noise columns to nodes table",
				Version:     222,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN noise_proto integer;`,
					`ALTER TABLE nodes ADD COLUMN noise_public_key bytea;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create index for interval_day column for project_bandwidth_daily_rollup",
				Version:     223,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS project_bandwidth_daily_rollup_interval_day_index ON project_bandwidth_daily_rollups ( interval_day ) ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Create user_settings table",
				Version:     224,
				Action: migrate.SQL{
					`CREATE TABLE user_settings (
						user_id bytea NOT NULL,
						session_minutes integer,
						PRIMARY KEY ( user_id )
					);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop unused column last_verification_reminder on users table",
				Version:     225,
				Action: migrate.SQL{
					`ALTER TABLE users DROP COLUMN last_verification_reminder;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add passphrase_prompt column to user_settings table",
				Version:     226,
				Action: migrate.SQL{
					`ALTER TABLE user_settings ADD COLUMN passphrase_prompt boolean;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "fix bucket_bandwidth_rollups primary key",
				Version:     227,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					alterPrimaryKey := true
					// for crdb lets check if key was already altered, for pg we will do migration always
					if db.Name() == tagsql.CockroachName {
						var primaryKey string
						err := db.QueryRowContext(ctx,
							`WITH constraints AS (SHOW CONSTRAINTS FROM bucket_bandwidth_rollups) SELECT details FROM constraints WHERE constraint_type = 'PRIMARY KEY';`,
						).Scan(&primaryKey)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}

						// alter primary key only if it was not adjusted manually
						alterPrimaryKey = primaryKey != "PRIMARY KEY (project_id ASC, bucket_name ASC, interval_start ASC, action ASC)"
					}

					if alterPrimaryKey {
						_, err := tx.ExecContext(ctx, `
							ALTER TABLE bucket_bandwidth_rollups DROP CONSTRAINT bucket_bandwidth_rollups_pk;
							ALTER TABLE bucket_bandwidth_rollups ADD CONSTRAINT bucket_bandwidth_rollups_pk PRIMARY KEY ( project_id, bucket_name, interval_start, action );
						`)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}
					}

					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "create new index on users table to improve get users queries.",
				Version:     228,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS users_email_status_index ON users ( normalized_email, status );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add debounce_limit column to nodes table",
				Version:     229,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN debounce_limit integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add onboarding columns to user_settings table",
				SeparateTx:  true,
				Version:     230,
				Action: migrate.SQL{
					`ALTER TABLE user_settings ADD COLUMN onboarding_start boolean NOT NULL DEFAULT true;`,
					`ALTER TABLE user_settings ADD COLUMN onboarding_end boolean NOT NULL DEFAULT true;`,
					`ALTER TABLE user_settings ADD COLUMN onboarding_step text;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns package_plan and purchased_package_at to stripe_customers",
				Version:     231,
				Action: migrate.SQL{
					`ALTER TABLE stripe_customers ADD COLUMN package_plan TEXT;`,
					`ALTER TABLE stripe_customers ADD COLUMN purchased_package_at timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create project_invitations table",
				Version:     232,
				Action: migrate.SQL{
					`CREATE TABLE project_invitations (
						project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
						email text NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( project_id, email )
					);`,
					`CREATE INDEX project_invitations_project_id_index ON project_invitations ( project_id );`,
					`CREATE INDEX project_invitations_email_index ON project_invitations ( email );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "make value_attributions.partner_id nullable",
				Version:     233,
				SeparateTx:  true,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions ALTER COLUMN partner_id DROP NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create index for owner_id column for projects",
				Version:     234,
				SeparateTx:  true,
				Action: migrate.SQL{
					`CREATE INDEX projects_owner_id_index ON projects ( owner_id )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add inviter_id column to project_invitations table",
				Version:     235,
				Action: migrate.SQL{
					`ALTER TABLE project_invitations ADD COLUMN inviter_id bytea REFERENCES users( id ) ON DELETE SET NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop partner_id columns",
				Version:     236,
				Action: migrate.SQL{
					`ALTER TABLE projects DROP COLUMN partner_id;`,
					`ALTER TABLE users DROP COLUMN partner_id;`,
					`ALTER TABLE api_keys DROP COLUMN partner_id;`,
					`ALTER TABLE bucket_metainfos DROP COLUMN partner_id;`,
					`ALTER TABLE value_attributions DROP COLUMN partner_id;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add back value_attributions.partner_id column",
				Version:     237,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions ADD COLUMN partner_id bytea DEFAULT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add node features",
				Version:     238,
				Action: migrate.SQL{
					`ALTER TABLE nodes ADD COLUMN features integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add default_placement to users/projects",
				Version:     239,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN default_placement integer;`,
					`ALTER TABLE projects ADD COLUMN default_placement integer;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create index on project_id column for project_members",
				Version:     240,
				Action: migrate.SQL{
					`CREATE INDEX project_members_project_id_index ON project_members ( project_id );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "create index on project_id column for project_members",
				Version:     241,
				Action: migrate.SQL{
					`CREATE TABLE node_tags (
						node_id bytea NOT NULL,
						name text NOT NULL,
						value bytea NOT NULL,
						signed_at timestamp with time zone NOT NULL,
						signer bytea NOT NULL,
						PRIMARY KEY ( node_id, name, signer ));`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add (indexed) placement to repair_queue",
				Version:     242,
				Action: migrate.SQL{
					`ALTER TABLE repair_queue ADD COLUMN placement integer;`,
					`CREATE INDEX repair_queue_placement_index ON repair_queue ( placement ) ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop index from bucket_metainfos",
				Version:     243,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) (err error) {
					if db.Name() == tagsql.CockroachName {
						_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS bucket_metainfos_project_id_name_key CASCADE`)
					} else {
						_, err = tx.ExecContext(ctx, `ALTER TABLE bucket_metainfos DROP CONSTRAINT IF EXISTS bucket_metainfos_project_id_name_key;`)
					}
					return ErrMigrate.Wrap(err)
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "alter bucket_metainfos primary key",
				Version:     244,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					alterPrimaryKey := true
					// for crdb lets check if key was already altered, for pg we will do migration always
					if db.Name() == tagsql.CockroachName {
						var primaryKey string
						err := db.QueryRowContext(ctx,
							`WITH constraints AS (SHOW CONSTRAINTS FROM bucket_metainfos) SELECT details FROM constraints WHERE constraint_type = 'PRIMARY KEY';`,
						).Scan(&primaryKey)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}

						// alter primary key only if it was not adjusted manually
						alterPrimaryKey = primaryKey != "PRIMARY KEY (project_id ASC, name ASC)"
					}

					if alterPrimaryKey {
						_, err := tx.ExecContext(ctx, `
							ALTER TABLE bucket_metainfos DROP CONSTRAINT bucket_metainfos_pkey;
							ALTER TABLE bucket_metainfos ADD CONSTRAINT bucket_metainfos_pkey PRIMARY KEY ( project_id, name );
						`)
						if err != nil {
							return ErrMigrate.Wrap(err)
						}
					}
					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "add index to bucket_storage_tallies",
				Version:     245,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS bucket_storage_tallies_interval_start_index ON bucket_storage_tallies ( interval_start );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to account freeze event for days until next freeze event",
				Version:     246,
				Action: migrate.SQL{
					`ALTER TABLE account_freeze_events ADD COLUMN days_till_escalation integer;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "back fill days_till_escalation column for account_freeze_events",
				SeparateTx:  true,
				Version:     247,
				Action: migrate.SQL{
					// current default days for billing warning(1)-billing freeze is 15,
					// for billing freeze(0)-violation freeze is 60.
					`UPDATE account_freeze_events SET days_till_escalation = 15 WHERE event = 1;`,
					`UPDATE account_freeze_events SET days_till_escalation = 60 WHERE event = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "remove type column from indices on nodes",
				Version:     248,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					storingClause := func(fields ...string) string {
						if db.impl == dbutil.Cockroach {
							return fmt.Sprintf("STORING (%s)", strings.Join(fields, ", "))
						}

						return ""
					}
					queries := [4]string{
						`CREATE INDEX IF NOT EXISTS  nodes_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index
							ON nodes (last_contact_success, free_disk, major, minor, patch, vetted_at)
							` + storingClause("last_net", "address", "last_ip_port") + `
							WHERE disqualified IS NULL AND
							unknown_audit_suspended IS NULL AND
							exit_initiated_at IS NULL AND
							release = true AND
							last_net != ''`,
						`CREATE INDEX IF NOT EXISTS  nodes_dis_unk_aud_exit_init_rel_last_cont_success_stored_index
							ON nodes (disqualified ASC, unknown_audit_suspended ASC, exit_initiated_at ASC, release ASC, last_contact_success DESC)
							` + storingClause("free_disk", "minor", "major", "patch", "vetted_at", "last_net", "address", "last_ip_port") + `
							WHERE disqualified IS NULL AND
							unknown_audit_suspended IS NULL AND
							exit_initiated_at IS NULL AND
							release = true`,
						`DROP INDEX IF EXISTS nodes_type_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index`,
						`DROP INDEX IF EXISTS nodes_dis_unk_aud_exit_init_rel_type_last_cont_success_stored_index`,
					}

					for _, query := range queries {
						_, err := tx.ExecContext(ctx, query)
						if err != nil {
							return err
						}
					}

					return nil
				}),
			},
			{
				DB:          &db.migrationDB,
				Description: "drop partner_id column",
				Version:     249,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions DROP COLUMN partner_id;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add versioning to bucket_metainfos",
				Version:     250,
				Action: migrate.SQL{
					`ALTER TABLE bucket_metainfos ADD COLUMN versioning INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "Add activation_code column to users table",
				Version:     251,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN activation_code TEXT;`,
					`ALTER TABLE users ADD COLUMN signup_id TEXT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add default versioning to project",
				Version:     252,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN default_versioning INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop type column from nodes",
				Version:     253,
				Action: migrate.SQL{
					`ALTER TABLE nodes DROP COLUMN type;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update default versioning for projects to match default versioning values for buckets DB",
				Version:     254,
				Action: migrate.SQL{
					`ALTER TABLE projects ALTER COLUMN default_versioning SET DEFAULT 1;`,
					`UPDATE projects SET default_versioning = 1 WHERE default_versioning = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add last_ip_port to node_events",
				Version:     255,
				Action: migrate.SQL{
					`ALTER TABLE node_events ADD COLUMN last_ip_port TEXT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add notice_dismissal column to user_settings to track dismissed notices",
				Version:     256,
				Action: migrate.SQL{
					`ALTER TABLE user_settings ADD COLUMN notice_dismissal jsonb NOT NULL DEFAULT '{}';`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns to handle user free trial",
				Version:     257,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN trial_notifications INTEGER NOT NULL DEFAULT 0;`,
					`ALTER TABLE users ADD COLUMN trial_expiration timestamp with time zone;`,
					`ALTER TABLE users ADD COLUMN upgrade_time timestamp with time zone;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add chain_id to storjscan payments table",
				Version:     258,
				Action: migrate.SQL{
					`ALTER TABLE storjscan_payments ADD COLUMN chain_id bigint NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "backfill storjscan chain_id column",
				Version:     259,
				Action: migrate.SQL{
					`UPDATE storjscan_payments SET chain_id = 1 WHERE chain_id = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update storjscan payments table index to include chain_id",
				Version:     260,
				Action: migrate.SQL{
					`DROP INDEX storjscan_payments_block_number_log_index_index;`,
					`CREATE INDEX storjscan_payments_chain_id_block_number_log_index_index ON storjscan_payments ( chain_id, block_number, log_index );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "(reverted migration) update storjscan payments table to use chain_id in primary key",
				Version:     261,
				Action:      migrate.SQL{},
			},
			{
				DB:          &db.migrationDB,
				Description: "update transaction source to specify chain type ",
				Version:     262,
				Action: migrate.SQL{
					`UPDATE billing_transactions SET source = 'ethereum' WHERE source = 'storjscan';`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add billing_customer_id column to stripe_customers",
				Version:     263,
				Action: migrate.SQL{
					`ALTER TABLE stripe_customers ADD COLUMN billing_customer_id TEXT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add created_by column to api_keys and bucket_metainfos to be populated with a user id value",
				Version:     264,
				Action: migrate.SQL{
					`ALTER TABLE api_keys ADD COLUMN created_by bytea REFERENCES users( id ) DEFAULT NULL;`,
					`ALTER TABLE bucket_metainfos ADD COLUMN created_by bytea REFERENCES users( id ) DEFAULT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add version column to api_keys table",
				Version:     265,
				Action: migrate.SQL{
					`ALTER TABLE api_keys ADD COLUMN version integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add prompted_for_versioning_beta column to projects to track if prompted for versioning beta opt-in",
				Version:     266,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN prompted_for_versioning_beta boolean NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index on trial_expiration column in users table",
				Version:     267,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS trial_expiration_index ON users (trial_expiration);`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add passphrase_enc and path_encryption columns to projects to control satellite-managed-passphrase projects",
				Version:     268,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN passphrase_enc bytea DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN path_encryption boolean NOT NULL DEFAULT true;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add role column to project_members to indicate the rights each member has",
				Version:     269,
				Action: migrate.SQL{
					`ALTER TABLE project_members ADD COLUMN role integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "index project_id where state = 0 in stripecoinpayments_invoice_project_records",
				Version:     270,
				Action: migrate.SQL{
					`CREATE INDEX stripecoinpayments_invoice_project_records_unbilled_project_id_index ON stripecoinpayments_invoice_project_records ( project_id ) WHERE state = 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to account freeze event for number of event notifications sent",
				Version:     271,
				Action: migrate.SQL{
					`ALTER TABLE account_freeze_events ADD COLUMN notifications_count integer NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add object_lock_enabled column to bucket_metainfos table",
				Version:     272,
				Action: migrate.SQL{
					`ALTER TABLE bucket_metainfos ADD COLUMN object_lock_enabled boolean NOT NULL DEFAULT false;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add per-operation rate limits to projects table",
				Version:     273,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN rate_limit_head integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN burst_limit_head integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN rate_limit_get integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN burst_limit_get integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN rate_limit_put integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN burst_limit_put integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN rate_limit_list integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN burst_limit_list integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN rate_limit_del integer DEFAULT NULL;`,
					`ALTER TABLE projects ADD COLUMN burst_limit_del integer DEFAULT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "populate per-operation rate limits from existing values",
				Version:     274,
				Action: migrate.SQL{
					`UPDATE projects SET rate_limit_head=rate_limit, burst_limit_head=burst_limit,
						rate_limit_get=rate_limit, burst_limit_get=burst_limit,
						rate_limit_put=rate_limit, burst_limit_put=burst_limit,
						rate_limit_list=rate_limit, burst_limit_list=burst_limit,
						rate_limit_del=rate_limit, burst_limit_del=burst_limit
						WHERE rate_limit IS NOT NULL OR burst_limit IS NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "avoid using on delete set null",
				Version:     275,
				Action: migrate.SQL{
					`ALTER TABLE project_invitations DROP CONSTRAINT project_invitations_inviter_id_fkey;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "avoid using on delete set null",
				Version:     276,
				Action: migrate.SQL{
					`ALTER TABLE project_invitations ADD CONSTRAINT project_invitations_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column, passphrase_enc_key_id, to projects",
				Version:     277,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN passphrase_enc_key_id INTEGER;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns, status_updated_at and final_invoice_generated, to users",
				Version:     278,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN status_updated_at TIMESTAMP WITH TIME ZONE;`,
					`ALTER TABLE users ADD COLUMN final_invoice_generated BOOLEAN NOT NULL DEFAULT FALSE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add columns to users table for handling change email address process",
				Version:     279,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN new_unverified_email TEXT DEFAULT NULL;`,
					`ALTER TABLE users ADD COLUMN email_change_verification_step INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "rename columns which are forbidden for spanner",
				Version:     280,
				Action: migrate.SQL{
					`ALTER TABLE nodes RENAME COLUMN hash TO commit_hash;`,
					`ALTER TABLE nodes RENAME COLUMN timestamp TO release_timestamp;`,
					`ALTER TABLE storjscan_payments RENAME COLUMN timestamp TO block_timestamp;`,
					`ALTER TABLE billing_transactions RENAME COLUMN timestamp TO tx_timestamp;`,
					`ALTER INDEX billing_transactions_timestamp_index RENAME TO billing_transactions_tx_timestamp_index;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add default retention columns to bucket_metainfos",
				Version:     281,
				Action: migrate.SQL{
					`ALTER TABLE bucket_metainfos ADD COLUMN default_retention_mode INTEGER;`,
					`ALTER TABLE bucket_metainfos ADD COLUMN default_retention_days INTEGER;`,
					`ALTER TABLE bucket_metainfos ADD COLUMN default_retention_years INTEGER;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add external_id column to users",
				Version:     282,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN external_id TEXT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add partial index to external_id",
				Version:     283,
				Action: migrate.SQL{
					`CREATE INDEX users_external_id_index ON users ( external_id ) WHERE external_id IS NOT NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to projects table to track the status of the project",
				Version:     284,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN status INTEGER DEFAULT 1;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop all nodes table indexes execept primary key",
				Version:     285,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS nodes_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index`,
					`DROP INDEX IF EXISTS nodes_dis_unk_aud_exit_init_rel_last_cont_success_stored_index`,
					`DROP INDEX IF EXISTS node_last_ip`,
					`DROP INDEX IF EXISTS nodes_dis_unk_off_exit_fin_last_success_index`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add hubspot_object_id column to users",
				Version:     286,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN hubspot_object_id TEXT`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add product_id column to bucket_storage_tallies, bucket_bandwidth_rollups, bucket_bandwidth_rollup_archive, project_bandwidth_daily_rollups",
				Version:     287,
				Action: migrate.SQL{
					`ALTER TABLE bucket_storage_tallies ADD COLUMN product_id INTEGER`,
					`ALTER TABLE bucket_bandwidth_rollups ADD COLUMN product_id INTEGER`,
					`ALTER TABLE bucket_bandwidth_rollup_archives ADD COLUMN product_id INTEGER`,
					`ALTER TABLE project_bandwidth_daily_rollups ADD COLUMN product_id INTEGER`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add rest api keys table",
				Version:     288,
				Action: migrate.SQL{
					`CREATE TABLE rest_api_keys (
						id bytea NOT NULL,
						user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
						token bytea NOT NULL,
						name text NOT NULL,
						expires_at timestamp with time zone,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( id ),
    					UNIQUE ( token )
					);`,
					`CREATE INDEX rest_api_keys_user_id_index ON rest_api_keys ( user_id );`,
					`CREATE INDEX rest_api_keys_name_index ON rest_api_keys ( name );`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop table storagenode_bandwidth_rollups_phase2",
				Version:     289,
				Action: migrate.SQL{
					`DROP TABLE IF EXISTS storagenode_bandwidth_rollups_phase2`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column to users table to track user kind",
				Version:     290,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN kind INTEGER NOT NULL DEFAULT 0;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update user kind to 1 (PRO) for paid tier users",
				Version:     291,
				Action: migrate.SQL{
					`UPDATE users SET kind = 1 WHERE paid_tier = true`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add domains table",
				Version:     292,
				Action: migrate.SQL{
					`CREATE TABLE domains (
						project_id bytea NOT NULL REFERENCES projects( id ),
						subdomain text NOT NULL,
						prefix text NOT NULL,
						access_id text NOT NULL,
						created_by bytea NOT NULL REFERENCES users( id ),
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( project_id, subdomain )
					)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add placement to value_attributions table",
				Version:     293,
				Action: migrate.SQL{
					`ALTER TABLE value_attributions ADD COLUMN placement INTEGER`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add tags column to bucket_metainfos table",
				Version:     294,
				Action: migrate.SQL{
					`ALTER TABLE bucket_metainfos ADD COLUMN tags bytea;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add api_key_tails table",
				Version:     295,
				Action: migrate.SQL{
					`CREATE TABLE api_key_tails (
						tail bytea NOT NULL,
						parent_tail bytea NOT NULL,
						caveat bytea NOT NULL,
						last_used timestamp with time zone NOT NULL,
						PRIMARY KEY ( tail )
					)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "update project path encryption to true",
				Version:     296,
				Action: migrate.SQL{
					`UPDATE projects SET path_encryption = true WHERE path_encryption = false`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add root_key_id column to api_key_tails table",
				Version:     297,
				Action: migrate.SQL{
					`ALTER TABLE api_key_tails ADD COLUMN root_key_id bytea REFERENCES api_keys( id ) ON DELETE CASCADE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "drop paid_tier column from users table",
				Version:     298,
				Action: migrate.SQL{
					`ALTER TABLE users DROP COLUMN paid_tier;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index to users table on status and status_updated_at",
				Version:     299,
				Action: migrate.SQL{
					`CREATE INDEX users_status_status_updated_at_index ON users ( status, status_updated_at ) WHERE users.status_updated_at is not NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add column indexed status_updated_at to projects with status",
				Version:     300,
				Action: migrate.SQL{
					`ALTER TABLE projects ADD COLUMN status_updated_at TIMESTAMP WITH TIME ZONE;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add index to projects table on status and status_updated_at",
				Version:     301,
				Action: migrate.SQL{
					`CREATE INDEX projects_status_status_updated_at_index ON projects ( status, status_updated_at ) WHERE projects.status_updated_at is not NULL ;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add entitlements table",
				Version:     302,
				Action: migrate.SQL{
					`CREATE TABLE entitlements (
						scope bytea NOT NULL,
						features jsonb NOT NULL DEFAULT '{}',
						updated_at timestamp with time zone NOT NULL,
						created_at timestamp with time zone NOT NULL,
						PRIMARY KEY ( scope )
					)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add bucket_migrations table",
				Version:     303,
				Action: migrate.SQL{
					`CREATE TABLE bucket_migrations (
						id bytea NOT NULL,
						project_id bytea NOT NULL REFERENCES projects( id ),
						bucket_name bytea NOT NULL,
						from_placement integer NOT NULL,
						to_placement integer NOT NULL,
						migration_type integer NOT NULL,
						state text NOT NULL,
						bytes_processed bigint NOT NULL DEFAULT 0,
						error_message text,
						created_at timestamp with time zone NOT NULL,
						updated_at timestamp with time zone NOT NULL,
						completed_at timestamp with time zone,
						PRIMARY KEY ( id )
					)`,
					`CREATE INDEX bucket_migrations_state_created_at_index ON bucket_migrations ( state, created_at )`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add tenant_id column to users",
				Version:     304,
				Action: migrate.SQL{
					`ALTER TABLE users ADD COLUMN tenant_id TEXT;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add indexes to users.tenant_id column",
				Version:     305,
				Action: migrate.SQL{
					`CREATE INDEX users_tenant_id_index ON users ( tenant_id ) WHERE tenant_id IS NOT NULL;`,
					`CREATE INDEX users_normalized_email_tenant_id_status_index ON users ( normalized_email, tenant_id, status ) WHERE users.tenant_id is not NULL;`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "remove unused GE tables",
				Version:     306,
				Action: migrate.SQL{
					`DROP INDEX IF EXISTS graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index`,
					`DROP TABLE IF EXISTS graceful_exit_progress`,
					`DROP TABLE IF EXISTS graceful_exit_segment_transfer_queue`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add bucket_eventing_configs table",
				Version:     307,
				Action: migrate.SQL{
					`CREATE TABLE bucket_eventing_configs (
						project_id bytea NOT NULL,
						bucket_name bytea NOT NULL,
						config_id text NOT NULL DEFAULT gen_random_uuid()::text,
						topic_name text NOT NULL,
						events text[] NOT NULL,
						filter_prefix bytea,
						filter_suffix bytea,
						created_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
						updated_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
						CONSTRAINT bucket_eventing_configs_bucket_fkey
							FOREIGN KEY (project_id, bucket_name)
							REFERENCES bucket_metainfos (project_id, name)
							ON DELETE CASCADE,
						PRIMARY KEY ( project_id, bucket_name )
					)`,
				},
			},
			{
				DB:          &db.migrationDB,
				Description: "add change_histories table",
				Version:     308,
				Action: migrate.SQL{
					`CREATE TABLE change_histories (
						id bytea NOT NULL,
						admin_email text NOT NULL,
						user_id bytea NOT NULL,
						project_id bytea,
						bucket_name bytea,
						item_type text NOT NULL,
						operation text NOT NULL,
						reason text NOT NULL,
						changes jsonb NOT NULL,
						timestamp timestamp with time zone NOT NULL DEFAULT current_timestamp,
						PRIMARY KEY ( id )
					)`,
					`CREATE INDEX change_history_user_id_timestamp_idx ON change_histories ( user_id, timestamp );`,
					`CREATE INDEX change_history_user_id_item_type_timestamp_idx ON change_histories ( user_id, item_type, timestamp );`,
					`CREATE INDEX change_history_project_id_item_type_timestamp_idx ON change_histories ( project_id, item_type, timestamp );`,
					`CREATE INDEX change_history_bucket_name_timestamp_idx ON change_histories ( bucket_name, timestamp );`,
				},
			},
			// NB: after updating testdata in `testdata`, run
			//     `go generate` to update `migratez.go`.
		},
	}
}
