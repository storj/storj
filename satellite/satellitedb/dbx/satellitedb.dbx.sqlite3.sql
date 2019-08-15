-- AUTOGENERATED BY gopkg.in/spacemonkeygo/dbx.v1
-- DO NOT EDIT
CREATE TABLE accounting_rollups (
	id INTEGER NOT NULL,
	node_id BLOB NOT NULL,
	start_time TIMESTAMP NOT NULL,
	put_total INTEGER NOT NULL,
	get_total INTEGER NOT NULL,
	get_audit_total INTEGER NOT NULL,
	get_repair_total INTEGER NOT NULL,
	put_repair_total INTEGER NOT NULL,
	at_rest_total REAL NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE accounting_timestamps (
	name TEXT NOT NULL,
	value TIMESTAMP NOT NULL,
	PRIMARY KEY ( name )
);
CREATE TABLE bucket_bandwidth_rollups (
	bucket_name BLOB NOT NULL,
	project_id BLOB NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INTEGER NOT NULL,
	action INTEGER NOT NULL,
	inline INTEGER NOT NULL,
	allocated INTEGER NOT NULL,
	settled INTEGER NOT NULL,
	PRIMARY KEY ( bucket_name, project_id, interval_start, action )
);
CREATE TABLE bucket_storage_tallies (
	bucket_name BLOB NOT NULL,
	project_id BLOB NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	inline INTEGER NOT NULL,
	remote INTEGER NOT NULL,
	remote_segments_count INTEGER NOT NULL,
	inline_segments_count INTEGER NOT NULL,
	object_count INTEGER NOT NULL,
	metadata_size INTEGER NOT NULL,
	PRIMARY KEY ( bucket_name, project_id, interval_start )
);
CREATE TABLE bucket_usages (
	id BLOB NOT NULL,
	bucket_id BLOB NOT NULL,
	rollup_end_time TIMESTAMP NOT NULL,
	remote_stored_data INTEGER NOT NULL,
	inline_stored_data INTEGER NOT NULL,
	remote_segments INTEGER NOT NULL,
	inline_segments INTEGER NOT NULL,
	objects INTEGER NOT NULL,
	metadata_size INTEGER NOT NULL,
	repair_egress INTEGER NOT NULL,
	get_egress INTEGER NOT NULL,
	audit_egress INTEGER NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE injuredsegments (
	path BLOB NOT NULL,
	data BLOB NOT NULL,
	attempted TIMESTAMP,
	PRIMARY KEY ( path )
);
CREATE TABLE irreparabledbs (
	segmentpath BLOB NOT NULL,
	segmentdetail BLOB NOT NULL,
	pieces_lost_count INTEGER NOT NULL,
	seg_damaged_unix_sec INTEGER NOT NULL,
	repair_attempt_count INTEGER NOT NULL,
	PRIMARY KEY ( segmentpath )
);
CREATE TABLE nodes (
	id BLOB NOT NULL,
	address TEXT NOT NULL,
	last_net TEXT NOT NULL,
	protocol INTEGER NOT NULL,
	type INTEGER NOT NULL,
	email TEXT NOT NULL,
	wallet TEXT NOT NULL,
	free_bandwidth INTEGER NOT NULL,
	free_disk INTEGER NOT NULL,
	major INTEGER NOT NULL,
	minor INTEGER NOT NULL,
	patch INTEGER NOT NULL,
	hash TEXT NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	release INTEGER NOT NULL,
	latency_90 INTEGER NOT NULL,
	audit_success_count INTEGER NOT NULL,
	total_audit_count INTEGER NOT NULL,
	uptime_success_count INTEGER NOT NULL,
	total_uptime_count INTEGER NOT NULL,
	piece_count INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	last_contact_success TIMESTAMP NOT NULL,
	last_contact_failure TIMESTAMP NOT NULL,
	contained INTEGER NOT NULL,
	disqualified TIMESTAMP,
	audit_reputation_alpha REAL NOT NULL,
	audit_reputation_beta REAL NOT NULL,
	uptime_reputation_alpha REAL NOT NULL,
	uptime_reputation_beta REAL NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE offers (
	id INTEGER NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	award_credit_in_cents INTEGER NOT NULL,
	invitee_credit_in_cents INTEGER NOT NULL,
	award_credit_duration_days INTEGER,
	invitee_credit_duration_days INTEGER,
	redeemable_cap INTEGER,
	expires_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP NOT NULL,
	status INTEGER NOT NULL,
	type INTEGER NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE pending_audits (
	node_id BLOB NOT NULL,
	piece_id BLOB NOT NULL,
	stripe_index INTEGER NOT NULL,
	share_size INTEGER NOT NULL,
	expected_share_hash BLOB NOT NULL,
	reverify_count INTEGER NOT NULL,
	path BLOB NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE projects (
	id BLOB NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	usage_limit INTEGER NOT NULL,
	partner_id BLOB,
	owner_id BLOB NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE registration_tokens (
	secret BLOB NOT NULL,
	owner_id BLOB,
	project_limit INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE reset_password_tokens (
	secret BLOB NOT NULL,
	owner_id BLOB NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE serial_numbers (
	id INTEGER NOT NULL,
	serial_number BLOB NOT NULL,
	bucket_id BLOB NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE storagenode_bandwidth_rollups (
	storagenode_id BLOB NOT NULL,
	interval_start TIMESTAMP NOT NULL,
	interval_seconds INTEGER NOT NULL,
	action INTEGER NOT NULL,
	allocated INTEGER NOT NULL,
	settled INTEGER NOT NULL,
	PRIMARY KEY ( storagenode_id, interval_start, action )
);
CREATE TABLE storagenode_storage_tallies (
	id INTEGER NOT NULL,
	node_id BLOB NOT NULL,
	interval_end_time TIMESTAMP NOT NULL,
	data_total REAL NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE users (
	id BLOB NOT NULL,
	email TEXT NOT NULL,
	full_name TEXT NOT NULL,
	short_name TEXT,
	password_hash BLOB NOT NULL,
	status INTEGER NOT NULL,
	partner_id BLOB,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE value_attributions (
	project_id BLOB NOT NULL,
	bucket_name BLOB NOT NULL,
	partner_id BLOB NOT NULL,
	last_updated TIMESTAMP NOT NULL,
	PRIMARY KEY ( project_id, bucket_name )
);
CREATE TABLE api_keys (
	id BLOB NOT NULL,
	project_id BLOB NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	head BLOB NOT NULL,
	name TEXT NOT NULL,
	secret BLOB NOT NULL,
	partner_id BLOB,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( head ),
	UNIQUE ( name, project_id )
);
CREATE TABLE bucket_metainfos (
	id BLOB NOT NULL,
	project_id BLOB NOT NULL REFERENCES projects( id ),
	name BLOB NOT NULL,
	partner_id BLOB,
	path_cipher INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	default_segment_size INTEGER NOT NULL,
	default_encryption_cipher_suite INTEGER NOT NULL,
	default_encryption_block_size INTEGER NOT NULL,
	default_redundancy_algorithm INTEGER NOT NULL,
	default_redundancy_share_size INTEGER NOT NULL,
	default_redundancy_required_shares INTEGER NOT NULL,
	default_redundancy_repair_shares INTEGER NOT NULL,
	default_redundancy_optimal_shares INTEGER NOT NULL,
	default_redundancy_total_shares INTEGER NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( name, project_id )
);
CREATE TABLE project_invoice_stamps (
	project_id BLOB NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	invoice_id BLOB NOT NULL,
	start_date TIMESTAMP NOT NULL,
	end_date TIMESTAMP NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( project_id, start_date, end_date ),
	UNIQUE ( invoice_id )
);
CREATE TABLE project_members (
	member_id BLOB NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	project_id BLOB NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( member_id, project_id )
);
CREATE TABLE used_serials (
	serial_number_id INTEGER NOT NULL REFERENCES serial_numbers( id ) ON DELETE CASCADE,
	storage_node_id BLOB NOT NULL,
	PRIMARY KEY ( serial_number_id, storage_node_id )
);
CREATE TABLE user_credits (
	id INTEGER NOT NULL,
	user_id BLOB NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	offer_id INTEGER NOT NULL REFERENCES offers( id ),
	referred_by BLOB REFERENCES users( id ) ON DELETE SET NULL,
	type TEXT NOT NULL,
	credits_earned_in_cents INTEGER NOT NULL,
	credits_used_in_cents INTEGER NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE user_payments (
	user_id BLOB NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	customer_id BLOB NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( user_id ),
	UNIQUE ( customer_id )
);
CREATE TABLE project_payments (
	id BLOB NOT NULL,
	project_id BLOB NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	payer_id BLOB NOT NULL REFERENCES user_payments( user_id ) ON DELETE CASCADE,
	payment_method_id BLOB NOT NULL,
	is_default INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL,
	PRIMARY KEY ( id )
);
CREATE INDEX bucket_name_project_id_interval_start_interval_seconds ON bucket_bandwidth_rollups ( bucket_name, project_id, interval_start, interval_seconds );
CREATE UNIQUE INDEX bucket_id_rollup ON bucket_usages ( bucket_id, rollup_end_time );
CREATE INDEX injuredsegments_attempted_index ON injuredsegments ( attempted );
CREATE INDEX node_last_ip ON nodes ( last_net );
CREATE UNIQUE INDEX serial_number ON serial_numbers ( serial_number );
CREATE INDEX serial_numbers_expires_at_index ON serial_numbers ( expires_at );
CREATE INDEX storagenode_id_interval_start_interval_seconds ON storagenode_bandwidth_rollups ( storagenode_id, interval_start, interval_seconds );
