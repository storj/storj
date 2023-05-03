// AUTOGENERATED BY migrategen.go
// DO NOT EDIT.

package satellitedb

import "storj.io/storj/private/migrate"

// testMigration returns migration that can be used for testing.
func (db *satelliteDB) testMigration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Testing setup",
				Version:     232,
				Action: migrate.SQL{`-- AUTOGENERATED BY storj.io/dbx
-- DO NOT EDIT
CREATE TABLE account_freeze_events (
	user_id bytea NOT NULL,
	event integer NOT NULL,
	limits jsonb,
	created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	PRIMARY KEY ( user_id, event )
);
CREATE TABLE accounting_rollups (
	node_id bytea NOT NULL,
	start_time timestamp with time zone NOT NULL,
	put_total bigint NOT NULL,
	get_total bigint NOT NULL,
	get_audit_total bigint NOT NULL,
	get_repair_total bigint NOT NULL,
	put_repair_total bigint NOT NULL,
	at_rest_total double precision NOT NULL,
	interval_end_time timestamp with time zone,
	PRIMARY KEY ( node_id, start_time )
);
CREATE TABLE accounting_timestamps (
	name text NOT NULL,
	value timestamp with time zone NOT NULL,
	PRIMARY KEY ( name )
);
CREATE TABLE billing_balances (
	user_id bytea NOT NULL,
	balance bigint NOT NULL,
	last_updated timestamp with time zone NOT NULL,
	PRIMARY KEY ( user_id )
);
CREATE TABLE billing_transactions (
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
);
CREATE TABLE bucket_bandwidth_rollups (
	bucket_name bytea NOT NULL,
	project_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	interval_seconds integer NOT NULL,
	action integer NOT NULL,
	inline bigint NOT NULL,
	allocated bigint NOT NULL,
	settled bigint NOT NULL,
	PRIMARY KEY ( project_id, bucket_name, interval_start, action )
);
CREATE TABLE bucket_bandwidth_rollup_archives (
	bucket_name bytea NOT NULL,
	project_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	interval_seconds integer NOT NULL,
	action integer NOT NULL,
	inline bigint NOT NULL,
	allocated bigint NOT NULL,
	settled bigint NOT NULL,
	PRIMARY KEY ( bucket_name, project_id, interval_start, action )
);
CREATE TABLE bucket_storage_tallies (
	bucket_name bytea NOT NULL,
	project_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	total_bytes bigint NOT NULL DEFAULT 0,
	inline bigint NOT NULL,
	remote bigint NOT NULL,
	total_segments_count integer NOT NULL DEFAULT 0,
	remote_segments_count integer NOT NULL,
	inline_segments_count integer NOT NULL,
	object_count integer NOT NULL,
	metadata_size bigint NOT NULL,
	PRIMARY KEY ( bucket_name, project_id, interval_start )
);
CREATE TABLE coinpayments_transactions (
	id text NOT NULL,
	user_id bytea NOT NULL,
	address text NOT NULL,
	amount_numeric bigint NOT NULL,
	received_numeric bigint NOT NULL,
	status integer NOT NULL,
	key text NOT NULL,
	timeout integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE graceful_exit_progress (
	node_id bytea NOT NULL,
	bytes_transferred bigint NOT NULL,
	pieces_transferred bigint NOT NULL DEFAULT 0,
	pieces_failed bigint NOT NULL DEFAULT 0,
	updated_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE graceful_exit_segment_transfer_queue (
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
);
CREATE TABLE nodes (
	id bytea NOT NULL,
	address text NOT NULL DEFAULT '',
	last_net text NOT NULL,
	last_ip_port text,
	country_code text,
	protocol integer NOT NULL DEFAULT 0,
	type integer NOT NULL DEFAULT 0,
	email text NOT NULL,
	wallet text NOT NULL,
	wallet_features text NOT NULL DEFAULT '',
	free_disk bigint NOT NULL DEFAULT -1,
	piece_count bigint NOT NULL DEFAULT 0,
	major bigint NOT NULL DEFAULT 0,
	minor bigint NOT NULL DEFAULT 0,
	patch bigint NOT NULL DEFAULT 0,
	hash text NOT NULL DEFAULT '',
	timestamp timestamp with time zone NOT NULL DEFAULT '0001-01-01 00:00:00+00',
	release boolean NOT NULL DEFAULT false,
	latency_90 bigint NOT NULL DEFAULT 0,
	vetted_at timestamp with time zone,
	created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	last_contact_success timestamp with time zone NOT NULL DEFAULT 'epoch',
	last_contact_failure timestamp with time zone NOT NULL DEFAULT 'epoch',
	disqualified timestamp with time zone,
	disqualification_reason integer,
	unknown_audit_suspended timestamp with time zone,
	offline_suspended timestamp with time zone,
	under_review timestamp with time zone,
	exit_initiated_at timestamp with time zone,
	exit_loop_completed_at timestamp with time zone,
	exit_finished_at timestamp with time zone,
	exit_success boolean NOT NULL DEFAULT false,
	contained timestamp with time zone,
	last_offline_email timestamp with time zone,
	last_software_update_email timestamp with time zone,
	noise_proto int,
	noise_public_key bytea,
	debounce_limit int NOT NULL DEFAULT 0,
	PRIMARY KEY ( id )
);
CREATE TABLE node_events (
	id bytea NOT NULL,
	email text NOT NULL,
	node_id bytea NOT NULL,
	event integer NOT NULL,
	created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	last_attempted timestamp with time zone,
	email_sent timestamp with time zone,
	PRIMARY KEY ( id )
);
CREATE TABLE node_api_versions (
	id bytea NOT NULL,
	api_version integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	updated_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE oauth_clients (
	id bytea NOT NULL,
	encrypted_secret bytea NOT NULL,
	redirect_url text NOT NULL,
	user_id bytea NOT NULL,
	app_name text NOT NULL,
	app_logo_url text NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE oauth_codes (
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
);
CREATE TABLE oauth_tokens (
	client_id bytea NOT NULL,
	user_id bytea NOT NULL,
	scope text NOT NULL,
	kind integer NOT NULL,
	token bytea NOT NULL,
	created_at timestamp with time zone NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( token )
);
CREATE TABLE peer_identities (
	node_id bytea NOT NULL,
	leaf_serial_number bytea NOT NULL,
	chain bytea NOT NULL,
	updated_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE projects (
	id bytea NOT NULL,
	public_id bytea,
	name text NOT NULL,
	description text NOT NULL,
	usage_limit bigint,
	bandwidth_limit bigint,
	user_specified_usage_limit bigint,
	user_specified_bandwidth_limit bigint,
	segment_limit bigint DEFAULT 1000000,
	rate_limit integer,
	burst_limit integer,
	max_buckets integer,
	partner_id bytea,
	user_agent bytea,
	owner_id bytea NOT NULL,
	salt bytea,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE project_bandwidth_daily_rollups (
	project_id bytea NOT NULL,
	interval_day date NOT NULL,
	egress_allocated bigint NOT NULL,
	egress_settled bigint NOT NULL,
	egress_dead bigint NOT NULL DEFAULT 0,
	PRIMARY KEY ( project_id, interval_day )
);
CREATE TABLE registration_tokens (
	secret bytea NOT NULL,
	owner_id bytea,
	project_limit integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE repair_queue (
	stream_id bytea NOT NULL,
	position bigint NOT NULL,
	attempted_at timestamp with time zone,
	updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	segment_health double precision NOT NULL DEFAULT 1,
	PRIMARY KEY ( stream_id, position )
);
CREATE TABLE reputations (
	id bytea NOT NULL,
	audit_success_count bigint NOT NULL DEFAULT 0,
	total_audit_count bigint NOT NULL DEFAULT 0,
	vetted_at timestamp with time zone,
	created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	disqualified timestamp with time zone,
	disqualification_reason integer,
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
);
CREATE TABLE reset_password_tokens (
	secret bytea NOT NULL,
	owner_id bytea NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE reverification_audits (
	node_id bytea NOT NULL,
	stream_id bytea NOT NULL,
	position bigint NOT NULL,
	piece_num integer NOT NULL,
	inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	last_attempt timestamp with time zone,
	reverify_count bigint NOT NULL DEFAULT 0,
	PRIMARY KEY ( node_id, stream_id, position )
);
CREATE TABLE revocations (
	revoked bytea NOT NULL,
	api_key_id bytea NOT NULL,
	PRIMARY KEY ( revoked )
);
CREATE TABLE segment_pending_audits (
	node_id bytea NOT NULL,
	stream_id bytea NOT NULL,
	position bigint NOT NULL,
	piece_id bytea NOT NULL,
	stripe_index bigint NOT NULL,
	share_size bigint NOT NULL,
	expected_share_hash bytea NOT NULL,
	reverify_count bigint NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE storagenode_bandwidth_rollups (
	storagenode_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	interval_seconds integer NOT NULL,
	action integer NOT NULL,
	allocated bigint DEFAULT 0,
	settled bigint NOT NULL,
	PRIMARY KEY ( storagenode_id, interval_start, action )
);
CREATE TABLE storagenode_bandwidth_rollup_archives (
	storagenode_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	interval_seconds integer NOT NULL,
	action integer NOT NULL,
	allocated bigint DEFAULT 0,
	settled bigint NOT NULL,
	PRIMARY KEY ( storagenode_id, interval_start, action )
);
CREATE TABLE storagenode_bandwidth_rollups_phase2 (
	storagenode_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	interval_seconds integer NOT NULL,
	action integer NOT NULL,
	allocated bigint DEFAULT 0,
	settled bigint NOT NULL,
	PRIMARY KEY ( storagenode_id, interval_start, action )
);
CREATE TABLE storagenode_payments (
	id bigserial NOT NULL,
	created_at timestamp with time zone NOT NULL,
	node_id bytea NOT NULL,
	period text NOT NULL,
	amount bigint NOT NULL,
	receipt text,
	notes text,
	PRIMARY KEY ( id )
);
CREATE TABLE storagenode_paystubs (
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
	distributed bigint NOT NULL,
	PRIMARY KEY ( period, node_id )
);
CREATE TABLE storagenode_storage_tallies (
	node_id bytea NOT NULL,
	interval_end_time timestamp with time zone NOT NULL,
	data_total double precision NOT NULL,
	PRIMARY KEY ( interval_end_time, node_id )
);
CREATE TABLE storjscan_payments (
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
);
CREATE TABLE storjscan_wallets (
	user_id bytea NOT NULL,
	wallet_address bytea NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( user_id, wallet_address )
);
CREATE TABLE stripe_customers (
	user_id bytea NOT NULL,
	customer_id text NOT NULL,
    package_plan text,
	purchased_package_at timestamp with time zone,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( user_id ),
	UNIQUE ( customer_id )
);
CREATE TABLE stripecoinpayments_invoice_project_records (
	id bytea NOT NULL,
	project_id bytea NOT NULL,
	storage double precision NOT NULL,
	egress bigint NOT NULL,
	objects bigint,
	segments bigint,
	period_start timestamp with time zone NOT NULL,
	period_end timestamp with time zone NOT NULL,
	state integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( project_id, period_start, period_end )
);
CREATE TABLE stripecoinpayments_tx_conversion_rates (
	tx_id text NOT NULL,
	rate_numeric double precision NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( tx_id )
);
CREATE TABLE users (
	id bytea NOT NULL,
	email text NOT NULL,
	normalized_email text NOT NULL,
	full_name text NOT NULL,
	short_name text,
	password_hash bytea NOT NULL,
	status integer NOT NULL,
	partner_id bytea,
	user_agent bytea,
	created_at timestamp with time zone NOT NULL,
	project_limit integer NOT NULL DEFAULT 0,
	project_bandwidth_limit bigint NOT NULL DEFAULT 0,
	project_storage_limit bigint NOT NULL DEFAULT 0,
	project_segment_limit bigint NOT NULL DEFAULT 0,
	paid_tier boolean NOT NULL DEFAULT false,
	position text,
	company_name text,
	company_size integer,
	working_on text,
	is_professional boolean NOT NULL DEFAULT false,
	employee_count text,
	have_sales_contact boolean NOT NULL DEFAULT false,
	mfa_enabled boolean NOT NULL DEFAULT false,
	mfa_secret_key text,
	mfa_recovery_codes text,
	signup_promo_code text,
	verification_reminders integer NOT NULL DEFAULT 0,
	failed_login_count integer,
	login_lockout_expiration timestamp with time zone,
	signup_captcha double precision,
	PRIMARY KEY ( id )
);
CREATE TABLE user_settings (
	user_id bytea NOT NULL,
	session_minutes integer,
    passphrase_prompt boolean,
    onboarding_start boolean NOT NULL DEFAULT true,
    onboarding_end boolean NOT NULL DEFAULT true,
    onboarding_step text,
	PRIMARY KEY ( user_id )
);
CREATE TABLE value_attributions (
	project_id bytea NOT NULL,
	bucket_name bytea NOT NULL,
	partner_id bytea NOT NULL,
	user_agent bytea,
	last_updated timestamp with time zone NOT NULL,
	PRIMARY KEY ( project_id, bucket_name )
);
CREATE TABLE verification_audits (
	inserted_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	stream_id bytea NOT NULL,
	position bigint NOT NULL,
	expires_at timestamp with time zone,
	encrypted_size integer NOT NULL,
	PRIMARY KEY ( inserted_at, stream_id, position )
);
CREATE TABLE webapp_sessions (
	id bytea NOT NULL,
	user_id bytea NOT NULL,
	ip_address text NOT NULL,
	user_agent text NOT NULL,
	status integer NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE api_keys (
	id bytea NOT NULL,
	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	head bytea NOT NULL,
	name text NOT NULL,
	secret bytea NOT NULL,
	partner_id bytea,
	user_agent bytea,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( head ),
	UNIQUE ( name, project_id )
);
CREATE TABLE bucket_metainfos (
	id bytea NOT NULL,
	project_id bytea NOT NULL REFERENCES projects( id ),
	name bytea NOT NULL,
	partner_id bytea,
	user_agent bytea,
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
	placement integer,
	PRIMARY KEY ( id ),
	UNIQUE ( project_id, name )
);
CREATE TABLE project_invitations (
	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	email text NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( project_id, email )
);
CREATE TABLE project_members (
	member_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( member_id, project_id )
);
CREATE TABLE stripecoinpayments_apply_balance_intents (
	tx_id text NOT NULL REFERENCES coinpayments_transactions( id ) ON DELETE CASCADE,
	state integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( tx_id )
);
CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups ( start_time ) ;
CREATE INDEX billing_transactions_timestamp_index ON billing_transactions ( timestamp ) ;
CREATE INDEX bucket_bandwidth_rollups_project_id_action_interval_index ON bucket_bandwidth_rollups ( project_id, action, interval_start ) ;
CREATE INDEX bucket_bandwidth_rollups_action_interval_project_id_index ON bucket_bandwidth_rollups ( action, interval_start, project_id ) ;
CREATE INDEX bucket_bandwidth_rollups_archive_project_id_action_interval_index ON bucket_bandwidth_rollup_archives ( project_id, action, interval_start ) ;
CREATE INDEX bucket_bandwidth_rollups_archive_action_interval_project_id_index ON bucket_bandwidth_rollup_archives ( action, interval_start, project_id ) ;
CREATE INDEX project_bandwidth_daily_rollup_interval_day_index ON project_bandwidth_daily_rollups ( interval_day ) ;
CREATE INDEX bucket_storage_tallies_project_id_interval_start_index ON bucket_storage_tallies ( project_id, interval_start ) ;
CREATE INDEX graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index ON graceful_exit_segment_transfer_queue ( node_id, durability_ratio, queued_at, finished_at, last_failed_at ) ;
CREATE INDEX node_last_ip ON nodes ( last_net ) ;
CREATE INDEX nodes_dis_unk_off_exit_fin_last_success_index ON nodes ( disqualified, unknown_audit_suspended, offline_suspended, exit_finished_at, last_contact_success ) ;
CREATE INDEX nodes_type_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index ON nodes ( type, last_contact_success, free_disk, major, minor, patch, vetted_at ) WHERE nodes.disqualified is NULL AND nodes.unknown_audit_suspended is NULL AND nodes.exit_initiated_at is NULL AND nodes.release = true AND nodes.last_net != '' ;
CREATE INDEX nodes_dis_unk_aud_exit_init_rel_type_last_cont_success_stored_index ON nodes ( disqualified, unknown_audit_suspended, exit_initiated_at, release, type, last_contact_success ) WHERE nodes.disqualified is NULL AND nodes.unknown_audit_suspended is NULL AND nodes.exit_initiated_at is NULL AND nodes.release = true ;
CREATE INDEX node_events_email_event_created_at_index ON node_events ( email, event, created_at ) WHERE node_events.email_sent is NULL ;
CREATE INDEX oauth_clients_user_id_index ON oauth_clients ( user_id ) ;
CREATE INDEX oauth_codes_user_id_index ON oauth_codes ( user_id ) ;
CREATE INDEX oauth_codes_client_id_index ON oauth_codes ( client_id ) ;
CREATE INDEX oauth_tokens_user_id_index ON oauth_tokens ( user_id ) ;
CREATE INDEX oauth_tokens_client_id_index ON oauth_tokens ( client_id ) ;
CREATE INDEX projects_public_id_index ON projects ( public_id ) ;
CREATE INDEX repair_queue_updated_at_index ON repair_queue ( updated_at ) ;
CREATE INDEX repair_queue_num_healthy_pieces_attempted_at_index ON repair_queue ( segment_health, attempted_at ) ;
CREATE INDEX reverification_audits_inserted_at_index ON reverification_audits ( inserted_at ) ;
CREATE INDEX storagenode_bandwidth_rollups_interval_start_index ON storagenode_bandwidth_rollups ( interval_start ) ;
CREATE INDEX storagenode_bandwidth_rollup_archives_interval_start_index ON storagenode_bandwidth_rollup_archives ( interval_start ) ;
CREATE INDEX storagenode_payments_node_id_period_index ON storagenode_payments ( node_id, period ) ;
CREATE INDEX storagenode_paystubs_node_id_index ON storagenode_paystubs ( node_id ) ;
CREATE INDEX storagenode_storage_tallies_node_id_index ON storagenode_storage_tallies ( node_id ) ;
CREATE INDEX storjscan_payments_block_number_log_index_index ON storjscan_payments ( block_number, log_index ) ;
CREATE INDEX storjscan_wallets_wallet_address_index ON storjscan_wallets ( wallet_address ) ;
CREATE INDEX webapp_sessions_user_id_index ON webapp_sessions ( user_id ) ;
CREATE INDEX users_email_status_index ON users ( normalized_email, status ) ;
CREATE INDEX project_invitations_project_id_index ON project_invitations ( project_id ) ;
CREATE INDEX project_invitations_email_index ON project_invitations ( email ) ;

`},
			},
		},
	}
}
