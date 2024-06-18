-- AUTOGENERATED BY storj.io/dbx
-- DO NOT EDIT
CREATE TABLE account_freeze_events (
	user_id BYTES(MAX) NOT NULL,
	event INT64 NOT NULL,
	limits JSON,
	days_till_escalation INT64,
	notifications_count INT64 NOT NULL DEFAULT (0),
	created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp)
) PRIMARY KEY ( user_id, event ) ;
CREATE TABLE accounting_rollups (
	node_id BYTES(MAX) NOT NULL,
	start_time TIMESTAMP NOT NULL,
	put_total INT64 NOT NULL,
	get_total INT64 NOT NULL,
	get_audit_total INT64 NOT NULL,
	get_repair_total INT64 NOT NULL,
	put_repair_total INT64 NOT NULL,
	at_rest_total FLOAT64 NOT NULL,
	interval_end_time TIMESTAMP
) PRIMARY KEY ( node_id, start_time ) ;
CREATE TABLE accounting_timestamps (
	name STRING(MAX) NOT NULL,
	value TIMESTAMP NOT NULL
) PRIMARY KEY ( name ) ;
CREATE TABLE billing_balances (
	user_id BYTES(MAX) NOT NULL,
	balance INT64 NOT NULL,
	last_updated TIMESTAMP NOT NULL
) PRIMARY KEY ( user_id ) ;
CREATE SEQUENCE billing_transactions_id OPTIONS (sequence_kind='bit_reversed_positive') ;
CREATE TABLE billing_transactions (
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
) PRIMARY KEY ( id ) ;
CREATE TABLE bucket_bandwidth_rollups (
	bucket_name BYTES(MAX) NOT NULL,
	project_id BYTES(MAX) NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INT64 NOT NULL,
	action INT64 NOT NULL,
	inline INT64 NOT NULL,
	allocated INT64 NOT NULL,
	settled INT64 NOT NULL
) PRIMARY KEY ( project_id, bucket_name, interval_start, action ) ;
CREATE TABLE bucket_bandwidth_rollup_archives (
	bucket_name BYTES(MAX) NOT NULL,
	project_id BYTES(MAX) NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INT64 NOT NULL,
	action INT64 NOT NULL,
	inline INT64 NOT NULL,
	allocated INT64 NOT NULL,
	settled INT64 NOT NULL
) PRIMARY KEY ( bucket_name, project_id, interval_start, action ) ;
CREATE TABLE bucket_storage_tallies (
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
) PRIMARY KEY ( bucket_name, project_id, interval_start ) ;
CREATE TABLE coinpayments_transactions (
	id STRING(MAX) NOT NULL,
	user_id BYTES(MAX) NOT NULL,
	address STRING(MAX) NOT NULL,
	amount_numeric INT64 NOT NULL,
	received_numeric INT64 NOT NULL,
	status INT64 NOT NULL,
	key STRING(MAX) NOT NULL,
	timeout INT64 NOT NULL,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( id ) ;
CREATE TABLE graceful_exit_progress (
	node_id BYTES(MAX) NOT NULL,
	bytes_transferred INT64 NOT NULL,
	pieces_transferred INT64 NOT NULL DEFAULT (0),
	pieces_failed INT64 NOT NULL DEFAULT (0),
	updated_at TIMESTAMP NOT NULL
) PRIMARY KEY ( node_id ) ;
CREATE TABLE graceful_exit_segment_transfer_queue (
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
) PRIMARY KEY ( node_id, stream_id, position, piece_num ) ;
CREATE TABLE nodes (
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
) PRIMARY KEY ( id ) ;
CREATE TABLE node_api_versions (
	id BYTES(MAX) NOT NULL,
	api_version INT64 NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
) PRIMARY KEY ( id ) ;
CREATE TABLE node_events (
	id BYTES(MAX) NOT NULL,
	email STRING(MAX) NOT NULL,
	last_ip_port STRING(MAX),
	node_id BYTES(MAX) NOT NULL,
	event INT64 NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
	last_attempted TIMESTAMP,
	email_sent TIMESTAMP
) PRIMARY KEY ( id ) ;
CREATE TABLE node_tags (
	node_id BYTES(MAX) NOT NULL,
	name STRING(MAX) NOT NULL,
	value BYTES(MAX) NOT NULL,
	signed_at TIMESTAMP NOT NULL,
	signer BYTES(MAX) NOT NULL
) PRIMARY KEY ( node_id, name, signer ) ;
CREATE TABLE oauth_clients (
	id BYTES(MAX) NOT NULL,
	encrypted_secret BYTES(MAX) NOT NULL,
	redirect_url STRING(MAX) NOT NULL,
	user_id BYTES(MAX) NOT NULL,
	app_name STRING(MAX) NOT NULL,
	app_logo_url STRING(MAX) NOT NULL
) PRIMARY KEY ( id ) ;
CREATE TABLE oauth_codes (
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
) PRIMARY KEY ( code ) ;
CREATE TABLE oauth_tokens (
	client_id BYTES(MAX) NOT NULL,
	user_id BYTES(MAX) NOT NULL,
	scope STRING(MAX) NOT NULL,
	kind INT64 NOT NULL,
	token BYTES(MAX) NOT NULL,
	created_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL
) PRIMARY KEY ( token ) ;
CREATE TABLE peer_identities (
	node_id BYTES(MAX) NOT NULL,
	leaf_serial_number BYTES(MAX) NOT NULL,
	chain BYTES(MAX) NOT NULL,
	updated_at TIMESTAMP NOT NULL
) PRIMARY KEY ( node_id ) ;
CREATE TABLE projects (
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
) PRIMARY KEY ( id ) ;
CREATE TABLE project_bandwidth_daily_rollups (
	project_id BYTES(MAX) NOT NULL,
	interval_day TIMESTAMP NOT NULL,
	egress_allocated INT64 NOT NULL,
	egress_settled INT64 NOT NULL,
	egress_dead INT64 NOT NULL DEFAULT (0)
) PRIMARY KEY ( project_id, interval_day ) ;
CREATE TABLE registration_tokens (
	secret BYTES(MAX) NOT NULL,
	owner_id BYTES(MAX),
	project_limit INT64 NOT NULL,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( secret ) ;
CREATE UNIQUE INDEX index_registration_tokens_owner_id ON registration_tokens (owner_id) ;
CREATE TABLE repair_queue (
	stream_id BYTES(MAX) NOT NULL,
	position INT64 NOT NULL,
	attempted_at TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
	inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
	segment_health FLOAT64 NOT NULL DEFAULT (1),
	placement INT64
) PRIMARY KEY ( stream_id, position ) ;
CREATE TABLE reputations (
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
) PRIMARY KEY ( id ) ;
CREATE TABLE reset_password_tokens (
	secret BYTES(MAX) NOT NULL,
	owner_id BYTES(MAX) NOT NULL,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( secret ) ;
CREATE UNIQUE INDEX index_reset_password_tokens_owner_id ON reset_password_tokens (owner_id) ;
CREATE TABLE reverification_audits (
	node_id BYTES(MAX) NOT NULL,
	stream_id BYTES(MAX) NOT NULL,
	position INT64 NOT NULL,
	piece_num INT64 NOT NULL,
	inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
	last_attempt TIMESTAMP,
	reverify_count INT64 NOT NULL DEFAULT (0)
) PRIMARY KEY ( node_id, stream_id, position ) ;
CREATE TABLE revocations (
	revoked BYTES(MAX) NOT NULL,
	api_key_id BYTES(MAX) NOT NULL
) PRIMARY KEY ( revoked ) ;
CREATE TABLE segment_pending_audits (
	node_id BYTES(MAX) NOT NULL,
	stream_id BYTES(MAX) NOT NULL,
	position INT64 NOT NULL,
	piece_id BYTES(MAX) NOT NULL,
	stripe_index INT64 NOT NULL,
	share_size INT64 NOT NULL,
	expected_share_hash BYTES(MAX) NOT NULL,
	reverify_count INT64 NOT NULL
) PRIMARY KEY ( node_id ) ;
CREATE TABLE storagenode_bandwidth_rollups (
	storagenode_id BYTES(MAX) NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INT64 NOT NULL,
	action INT64 NOT NULL,
	allocated INT64 DEFAULT (0),
	settled INT64 NOT NULL
) PRIMARY KEY ( storagenode_id, interval_start, action ) ;
CREATE TABLE storagenode_bandwidth_rollup_archives (
	storagenode_id BYTES(MAX) NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INT64 NOT NULL,
	action INT64 NOT NULL,
	allocated INT64 DEFAULT (0),
	settled INT64 NOT NULL
) PRIMARY KEY ( storagenode_id, interval_start, action ) ;
CREATE TABLE storagenode_bandwidth_rollups_phase2 (
	storagenode_id BYTES(MAX) NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INT64 NOT NULL,
	action INT64 NOT NULL,
	allocated INT64 DEFAULT (0),
	settled INT64 NOT NULL
) PRIMARY KEY ( storagenode_id, interval_start, action ) ;
CREATE SEQUENCE storagenode_payments_id OPTIONS (sequence_kind='bit_reversed_positive') ;
CREATE TABLE storagenode_payments (
	id INT64 NOT NULL DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE storagenode_payments_id)),
	created_at TIMESTAMP NOT NULL,
	node_id BYTES(MAX) NOT NULL,
	period STRING(MAX) NOT NULL,
	amount INT64 NOT NULL,
	receipt STRING(MAX),
	notes STRING(MAX)
) PRIMARY KEY ( id ) ;
CREATE TABLE storagenode_paystubs (
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
) PRIMARY KEY ( period, node_id ) ;
CREATE TABLE storagenode_storage_tallies (
	node_id BYTES(MAX) NOT NULL,
	interval_end_time TIMESTAMP NOT NULL,
	data_total FLOAT64 NOT NULL
) PRIMARY KEY ( interval_end_time, node_id ) ;
CREATE TABLE storjscan_payments (
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
) PRIMARY KEY ( block_hash, log_index ) ;
CREATE TABLE storjscan_wallets (
	user_id BYTES(MAX) NOT NULL,
	wallet_address BYTES(MAX) NOT NULL,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( user_id, wallet_address ) ;
CREATE TABLE stripe_customers (
	user_id BYTES(MAX) NOT NULL,
	customer_id STRING(MAX) NOT NULL,
	billing_customer_id STRING(MAX),
	package_plan STRING(MAX),
	purchased_package_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( user_id ) ;
CREATE UNIQUE INDEX index_stripe_customers_customer_id ON stripe_customers (customer_id) ;
CREATE TABLE stripecoinpayments_invoice_project_records (
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
) PRIMARY KEY ( id ) ;
CREATE UNIQUE INDEX index_stripecoinpayments_invoice_project_records_project_id ON stripecoinpayments_invoice_project_records (project_id) ;
CREATE TABLE stripecoinpayments_tx_conversion_rates (
	tx_id STRING(MAX) NOT NULL,
	rate_numeric FLOAT64 NOT NULL,
	created_at TIMESTAMP NOT NULL
) PRIMARY KEY ( tx_id ) ;
CREATE TABLE users (
	id BYTES(MAX) NOT NULL,
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
) PRIMARY KEY ( id ) ;
CREATE TABLE user_settings (
	user_id BYTES(MAX) NOT NULL,
	session_minutes INT64,
	passphrase_prompt BOOL,
	onboarding_start BOOL NOT NULL DEFAULT (true),
	onboarding_end BOOL NOT NULL DEFAULT (true),
	onboarding_step STRING(MAX),
	notice_dismissal JSON NOT NULL DEFAULT (JSON "{}")
) PRIMARY KEY ( user_id ) ;
CREATE TABLE value_attributions (
	project_id BYTES(MAX) NOT NULL,
	bucket_name BYTES(MAX) NOT NULL,
	user_agent BYTES(MAX),
	last_updated TIMESTAMP NOT NULL
) PRIMARY KEY ( project_id, bucket_name ) ;
CREATE TABLE verification_audits (
	inserted_at TIMESTAMP NOT NULL DEFAULT (current_timestamp),
	stream_id BYTES(MAX) NOT NULL,
	position INT64 NOT NULL,
	expires_at TIMESTAMP,
	encrypted_size INT64 NOT NULL
) PRIMARY KEY ( inserted_at, stream_id, position ) ;
CREATE TABLE webapp_sessions (
	id BYTES(MAX) NOT NULL,
	user_id BYTES(MAX) NOT NULL,
	ip_address STRING(MAX) NOT NULL,
	user_agent STRING(MAX) NOT NULL,
	status INT64 NOT NULL,
	expires_at TIMESTAMP NOT NULL
) PRIMARY KEY ( id ) ;
CREATE TABLE api_keys (
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
) PRIMARY KEY ( id ) ;
CREATE UNIQUE INDEX index_api_keys_head ON api_keys (head) ;
CREATE UNIQUE INDEX index_api_keys_name ON api_keys (name) ;
CREATE TABLE bucket_metainfos (
	id BYTES(MAX) NOT NULL,
	project_id BYTES(MAX) NOT NULL,
	name BYTES(MAX) NOT NULL,
	user_agent BYTES(MAX),
	versioning INT64 NOT NULL DEFAULT (0),
	object_lock_enabled BOOL NOT NULL DEFAULT (false),
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
) PRIMARY KEY ( project_id, name ) ;
CREATE TABLE project_invitations (
	project_id BYTES(MAX) NOT NULL,
	email STRING(MAX) NOT NULL,
	inviter_id BYTES(MAX),
	created_at TIMESTAMP NOT NULL,
	CONSTRAINT project_invitations_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE ,
	CONSTRAINT project_invitations_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES users (id) ON DELETE CASCADE 
) PRIMARY KEY ( project_id, email ) ;
CREATE TABLE project_members (
	member_id BYTES(MAX) NOT NULL,
	project_id BYTES(MAX) NOT NULL,
	role INT64 NOT NULL DEFAULT (0),
	created_at TIMESTAMP NOT NULL,
	CONSTRAINT project_members_member_id_fkey FOREIGN KEY (member_id) REFERENCES users (id) ON DELETE CASCADE ,
	CONSTRAINT project_members_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE 
) PRIMARY KEY ( member_id, project_id ) ;
CREATE TABLE stripecoinpayments_apply_balance_intents (
	tx_id STRING(MAX) NOT NULL,
	state INT64 NOT NULL,
	created_at TIMESTAMP NOT NULL,
	CONSTRAINT stripecoinpayments_apply_balance_intents_tx_id_fkey FOREIGN KEY (tx_id) REFERENCES coinpayments_transactions (id) ON DELETE CASCADE 
) PRIMARY KEY ( tx_id ) ;
CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups ( start_time ) ;
CREATE INDEX billing_transactions_tx_timestamp_index ON billing_transactions ( tx_timestamp ) ;
CREATE INDEX bucket_bandwidth_rollups_project_id_action_interval_index ON bucket_bandwidth_rollups ( project_id, action, interval_start ) ;
CREATE INDEX bucket_bandwidth_rollups_action_interval_project_id_index ON bucket_bandwidth_rollups ( action, interval_start, project_id ) ;
CREATE INDEX bucket_bandwidth_rollups_archive_project_id_action_interval_index ON bucket_bandwidth_rollup_archives ( project_id, action, interval_start ) ;
CREATE INDEX bucket_bandwidth_rollups_archive_action_interval_project_id_index ON bucket_bandwidth_rollup_archives ( action, interval_start, project_id ) ;
CREATE INDEX bucket_storage_tallies_project_id_interval_start_index ON bucket_storage_tallies ( project_id, interval_start ) ;
CREATE INDEX bucket_storage_tallies_interval_start_index ON bucket_storage_tallies ( interval_start ) ;
CREATE INDEX graceful_exit_segment_transfer_nid_dr_qa_fa_lfa_index ON graceful_exit_segment_transfer_queue ( node_id, durability_ratio, queued_at, finished_at, last_failed_at ) ;
CREATE INDEX node_last_ip ON nodes ( last_net ) ;
CREATE INDEX nodes_dis_unk_off_exit_fin_last_success_index ON nodes ( disqualified, unknown_audit_suspended, offline_suspended, exit_finished_at, last_contact_success ) ;
CREATE INDEX nodes_last_cont_success_free_disk_ma_mi_patch_vetted_partial_index ON nodes ( last_contact_success, free_disk, major, minor, patch, vetted_at ) ;
CREATE INDEX nodes_dis_unk_aud_exit_init_rel_last_cont_success_stored_index ON nodes ( disqualified, unknown_audit_suspended, exit_initiated_at, release, last_contact_success ) ;
CREATE INDEX node_events_email_event_created_at_index ON node_events ( email, event, created_at ) ;
CREATE INDEX oauth_clients_user_id_index ON oauth_clients ( user_id ) ;
CREATE INDEX oauth_codes_user_id_index ON oauth_codes ( user_id ) ;
CREATE INDEX oauth_codes_client_id_index ON oauth_codes ( client_id ) ;
CREATE INDEX oauth_tokens_user_id_index ON oauth_tokens ( user_id ) ;
CREATE INDEX oauth_tokens_client_id_index ON oauth_tokens ( client_id ) ;
CREATE INDEX projects_public_id_index ON projects ( public_id ) ;
CREATE INDEX projects_owner_id_index ON projects ( owner_id ) ;
CREATE INDEX project_bandwidth_daily_rollup_interval_day_index ON project_bandwidth_daily_rollups ( interval_day ) ;
CREATE INDEX repair_queue_updated_at_index ON repair_queue ( updated_at ) ;
CREATE INDEX repair_queue_num_healthy_pieces_attempted_at_index ON repair_queue ( segment_health, attempted_at ) ;
CREATE INDEX repair_queue_placement_index ON repair_queue ( placement ) ;
CREATE INDEX reverification_audits_inserted_at_index ON reverification_audits ( inserted_at ) ;
CREATE INDEX storagenode_bandwidth_rollups_interval_start_index ON storagenode_bandwidth_rollups ( interval_start ) ;
CREATE INDEX storagenode_bandwidth_rollup_archives_interval_start_index ON storagenode_bandwidth_rollup_archives ( interval_start ) ;
CREATE INDEX storagenode_payments_node_id_period_index ON storagenode_payments ( node_id, period ) ;
CREATE INDEX storagenode_paystubs_node_id_index ON storagenode_paystubs ( node_id ) ;
CREATE INDEX storagenode_storage_tallies_node_id_index ON storagenode_storage_tallies ( node_id ) ;
CREATE INDEX storjscan_payments_chain_id_block_number_log_index_index ON storjscan_payments ( chain_id, block_number, log_index ) ;
CREATE INDEX storjscan_wallets_wallet_address_index ON storjscan_wallets ( wallet_address ) ;
CREATE INDEX stripecoinpayments_invoice_project_records_unbilled_project_id_index ON stripecoinpayments_invoice_project_records ( project_id ) ;
CREATE INDEX users_email_status_index ON users ( normalized_email, status ) ;
CREATE INDEX trial_expiration_index ON users ( trial_expiration ) ;
CREATE INDEX webapp_sessions_user_id_index ON webapp_sessions ( user_id ) ;
CREATE INDEX project_invitations_project_id_index ON project_invitations ( project_id ) ;
CREATE INDEX project_invitations_email_index ON project_invitations ( email ) ;
CREATE INDEX project_members_project_id_index ON project_members ( project_id )