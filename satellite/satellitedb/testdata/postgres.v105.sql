-- AUTOGENERATED BY storj.io/dbx
-- DO NOT EDIT
CREATE TABLE accounting_rollups (
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
);
CREATE TABLE accounting_timestamps (
	name text NOT NULL,
	value timestamp with time zone NOT NULL,
	PRIMARY KEY ( name )
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
	PRIMARY KEY ( bucket_name, project_id, interval_start, action )
);
CREATE TABLE bucket_storage_tallies (
	bucket_name bytea NOT NULL,
	project_id bytea NOT NULL,
	interval_start timestamp with time zone NOT NULL,
	inline bigint NOT NULL,
	remote bigint NOT NULL,
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
	amount bytea NOT NULL,
	received bytea NOT NULL,
	status integer NOT NULL,
	key text NOT NULL,
	timeout integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE consumed_serials (
	storage_node_id bytea NOT NULL,
	serial_number bytea NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( storage_node_id, serial_number )
);
CREATE TABLE coupons (
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
);
CREATE TABLE coupon_usages (
	coupon_id bytea NOT NULL,
	amount bigint NOT NULL,
	status integer NOT NULL,
	period timestamp with time zone NOT NULL,
	PRIMARY KEY ( coupon_id, period )
);
CREATE TABLE credits (
	user_id bytea NOT NULL,
	transaction_id text NOT NULL,
	amount bigint NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( transaction_id )
);
CREATE TABLE credits_spendings (
	id bytea NOT NULL,
	user_id bytea NOT NULL,
	project_id bytea NOT NULL,
	amount bigint NOT NULL,
	status integer NOT NULL,
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
CREATE TABLE graceful_exit_transfer_queue (
	node_id bytea NOT NULL,
	path bytea NOT NULL,
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
	PRIMARY KEY ( node_id, path, piece_num )
);
CREATE TABLE injuredsegments (
	path bytea NOT NULL,
	data bytea NOT NULL,
	attempted timestamp with time zone,
	num_healthy_pieces integer NOT NULL DEFAULT 52,
	PRIMARY KEY ( path )
);
CREATE TABLE irreparabledbs (
	segmentpath bytea NOT NULL,
	segmentdetail bytea NOT NULL,
	pieces_lost_count bigint NOT NULL,
	seg_damaged_unix_sec bigint NOT NULL,
	repair_attempt_count bigint NOT NULL,
	PRIMARY KEY ( segmentpath )
);
CREATE TABLE nodes (
	id bytea NOT NULL,
	address text NOT NULL DEFAULT '',
	last_net text NOT NULL,
	last_ip_port text,
	protocol integer NOT NULL DEFAULT 0,
	type integer NOT NULL DEFAULT 0,
	email text NOT NULL,
	wallet text NOT NULL,
	free_disk bigint NOT NULL DEFAULT -1,
	piece_count bigint NOT NULL DEFAULT 0,
	major bigint NOT NULL DEFAULT 0,
	minor bigint NOT NULL DEFAULT 0,
	patch bigint NOT NULL DEFAULT 0,
	hash text NOT NULL DEFAULT '',
	timestamp timestamp with time zone NOT NULL DEFAULT '0001-01-01 00:00:00+00',
	release boolean NOT NULL DEFAULT false,
	latency_90 bigint NOT NULL DEFAULT 0,
	audit_success_count bigint NOT NULL DEFAULT 0,
	total_audit_count bigint NOT NULL DEFAULT 0,
	vetted_at timestamp with time zone,
	uptime_success_count bigint NOT NULL,
	total_uptime_count bigint NOT NULL,
	created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	updated_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
	last_contact_success timestamp with time zone NOT NULL DEFAULT 'epoch',
	last_contact_failure timestamp with time zone NOT NULL DEFAULT 'epoch',
	contained boolean NOT NULL DEFAULT false,
	disqualified timestamp with time zone,
	suspended timestamp with time zone,
	audit_reputation_alpha double precision NOT NULL DEFAULT 1,
	audit_reputation_beta double precision NOT NULL DEFAULT 0,
	unknown_audit_reputation_alpha double precision NOT NULL DEFAULT 1,
	unknown_audit_reputation_beta double precision NOT NULL DEFAULT 0,
	uptime_reputation_alpha double precision NOT NULL DEFAULT 1,
	uptime_reputation_beta double precision NOT NULL DEFAULT 0,
	exit_initiated_at timestamp with time zone,
	exit_loop_completed_at timestamp with time zone,
	exit_finished_at timestamp with time zone,
	exit_success boolean NOT NULL DEFAULT false,
	PRIMARY KEY ( id )
);
CREATE TABLE nodes_offline_times (
	node_id bytea NOT NULL,
	tracked_at timestamp with time zone NOT NULL,
	seconds integer NOT NULL,
	PRIMARY KEY ( node_id, tracked_at )
);
CREATE TABLE offers (
	id serial NOT NULL,
	name text NOT NULL,
	description text NOT NULL,
	award_credit_in_cents integer NOT NULL DEFAULT 0,
	invitee_credit_in_cents integer NOT NULL DEFAULT 0,
	award_credit_duration_days integer,
	invitee_credit_duration_days integer,
	redeemable_cap integer,
	expires_at timestamp with time zone NOT NULL,
	created_at timestamp with time zone NOT NULL,
	status integer NOT NULL,
	type integer NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE peer_identities (
	node_id bytea NOT NULL,
	leaf_serial_number bytea NOT NULL,
	chain bytea NOT NULL,
	updated_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE pending_audits (
	node_id bytea NOT NULL,
	piece_id bytea NOT NULL,
	stripe_index bigint NOT NULL,
	share_size bigint NOT NULL,
	expected_share_hash bytea NOT NULL,
	reverify_count bigint NOT NULL,
	path bytea NOT NULL,
	PRIMARY KEY ( node_id )
);
CREATE TABLE pending_serial_queue (
	storage_node_id bytea NOT NULL,
	bucket_id bytea NOT NULL,
	serial_number bytea NOT NULL,
	action integer NOT NULL,
	settled bigint NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( storage_node_id, bucket_id, serial_number )
);
CREATE TABLE projects (
	id bytea NOT NULL,
	name text NOT NULL,
	description text NOT NULL,
	usage_limit bigint NOT NULL DEFAULT 0,
	rate_limit integer,
	partner_id bytea,
	owner_id bytea NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE registration_tokens (
	secret bytea NOT NULL,
	owner_id bytea,
	project_limit integer NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE reported_serials (
	expires_at timestamp with time zone NOT NULL,
	storage_node_id bytea NOT NULL,
	bucket_id bytea NOT NULL,
	action integer NOT NULL,
	serial_number bytea NOT NULL,
	settled bigint NOT NULL,
	observed_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( expires_at, storage_node_id, bucket_id, action, serial_number )
);
CREATE TABLE reset_password_tokens (
	secret bytea NOT NULL,
	owner_id bytea NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( secret ),
	UNIQUE ( owner_id )
);
CREATE TABLE serial_numbers (
	id serial NOT NULL,
	serial_number bytea NOT NULL,
	bucket_id bytea NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
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
	PRIMARY KEY ( period, node_id )
);
CREATE TABLE storagenode_storage_tallies (
	node_id bytea NOT NULL,
	interval_end_time timestamp with time zone NOT NULL,
	data_total double precision NOT NULL,
	PRIMARY KEY ( interval_end_time, node_id )
);
CREATE TABLE stripe_customers (
	user_id bytea NOT NULL,
	customer_id text NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( user_id ),
	UNIQUE ( customer_id )
);
CREATE TABLE stripecoinpayments_invoice_project_records (
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
);
CREATE TABLE stripecoinpayments_tx_conversion_rates (
	tx_id text NOT NULL,
	rate bytea NOT NULL,
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
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id )
);
CREATE TABLE value_attributions (
	project_id bytea NOT NULL,
	bucket_name bytea NOT NULL,
	partner_id bytea NOT NULL,
	last_updated timestamp with time zone NOT NULL,
	PRIMARY KEY ( project_id, bucket_name )
);
CREATE TABLE api_keys (
	id bytea NOT NULL,
	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	head bytea NOT NULL,
	name text NOT NULL,
	secret bytea NOT NULL,
	partner_id bytea,
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
	PRIMARY KEY ( id ),
	UNIQUE ( name, project_id )
);
CREATE TABLE project_invoice_stamps (
	project_id bytea NOT NULL REFERENCES projects( id ) ON DELETE CASCADE,
	invoice_id bytea NOT NULL,
	start_date timestamp with time zone NOT NULL,
	end_date timestamp with time zone NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( project_id, start_date, end_date ),
	UNIQUE ( invoice_id )
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
CREATE TABLE used_serials (
	serial_number_id integer NOT NULL REFERENCES serial_numbers( id ) ON DELETE CASCADE,
	storage_node_id bytea NOT NULL,
	PRIMARY KEY ( serial_number_id, storage_node_id )
);
CREATE TABLE user_credits (
	id serial NOT NULL,
	user_id bytea NOT NULL REFERENCES users( id ) ON DELETE CASCADE,
	offer_id integer NOT NULL REFERENCES offers( id ),
	referred_by bytea REFERENCES users( id ) ON DELETE SET NULL,
	type text NOT NULL,
	credits_earned_in_cents integer NOT NULL,
	credits_used_in_cents integer NOT NULL,
	expires_at timestamp with time zone NOT NULL,
	created_at timestamp with time zone NOT NULL,
	PRIMARY KEY ( id ),
	UNIQUE ( id, offer_id )
);
CREATE INDEX accounting_rollups_start_time_index ON accounting_rollups ( start_time );
CREATE INDEX bucket_bandwidth_rollups_action_interval_project_id_index ON bucket_bandwidth_rollups ( action, interval_start, project_id );
CREATE INDEX consumed_serials_expires_at_index ON consumed_serials ( expires_at );
CREATE INDEX injuredsegments_attempted_index ON injuredsegments ( attempted );
CREATE INDEX injuredsegments_num_healthy_pieces_index ON injuredsegments ( num_healthy_pieces );
CREATE INDEX node_last_ip ON nodes ( last_net );
CREATE INDEX nodes_offline_times_node_id_index ON nodes_offline_times ( node_id );
CREATE UNIQUE INDEX serial_number_index ON serial_numbers ( serial_number );
CREATE INDEX serial_numbers_expires_at_index ON serial_numbers ( expires_at );
CREATE INDEX storagenode_payments_node_id_period_index ON storagenode_payments ( node_id, period );
CREATE INDEX storagenode_paystubs_node_id_index ON storagenode_paystubs ( node_id );
CREATE INDEX storagenode_storage_tallies_node_id_index ON storagenode_storage_tallies ( node_id );
CREATE UNIQUE INDEX credits_earned_user_id_offer_id ON user_credits ( id, offer_id );

INSERT INTO "accounting_rollups"("id", "node_id", "start_time", "put_total", "get_total", "get_audit_total", "get_repair_total", "put_repair_total", "at_rest_total") VALUES (1, E'\\367M\\177\\251]t/\\022\\256\\214\\265\\025\\224\\204:\\217\\212\\0102<\\321\\374\\020&\\271Qc\\325\\261\\354\\246\\233'::bytea, '2019-02-09 00:00:00+00', 1000, 2000, 3000, 4000, 0, 5000);

INSERT INTO "accounting_timestamps" VALUES ('LastAtRestTally', '0001-01-01 00:00:00+00');
INSERT INTO "accounting_timestamps" VALUES ('LastRollup', '0001-01-01 00:00:00+00');
INSERT INTO "accounting_timestamps" VALUES ('LastBandwidthTally', '0001-01-01 00:00:00+00');

INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\153\\313\\233\\074\\327\\177\\136\\070\\346\\001', '127.0.0.1:55516', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 0, 5, 0, 5, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, NULL, 50, 0, 1, 0, 100, 5, false);
INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', '127.0.0.1:55518', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 0, 0, 3, 3, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, NULL, 50, 0, 1, 0, 100, 0, false);
INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014', '127.0.0.1:55517', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 0, 0, 0, 0, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, NULL, 50, 0, 1, 0, 100, 0, false);
INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\015', '127.0.0.1:55519', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 1, 2, 1, 2, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, NULL, 50, 0, 1, 0, 100, 1, false);
INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', '127.0.0.1:55520', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 300, 400, 300, 400, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, NULL, 300, 0, 1, 0, 300, 100, false);
INSERT INTO "nodes"("id", "address", "last_net", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\154\\313\\233\\074\\327\\177\\136\\070\\346\\001', '127.0.0.1:55516', '', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 0, 5, 0, 5, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, '2019-02-14 08:07:31.028103+00', 50, 0, 75, 25, 100, 5, false);

INSERT INTO "users"("id", "full_name", "short_name", "email", "normalized_email", "password_hash", "status", "partner_id", "created_at") VALUES (E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 'Noahson', 'William', '1email1@mail.test', '1EMAIL1@MAIL.TEST', E'some_readable_hash'::bytea, 1, NULL, '2019-02-14 08:28:24.614594+00');
INSERT INTO "projects"("id", "name", "description", "usage_limit", "partner_id", "owner_id", "created_at") VALUES (E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, 'ProjectName', 'projects description', 0, NULL, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2019-02-14 08:28:24.254934+00');

INSERT INTO "projects"("id", "name", "description", "usage_limit", "partner_id", "owner_id", "created_at") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, 'projName1', 'Test project 1', 0, NULL, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2019-02-14 08:28:24.636949+00');
INSERT INTO "project_members"("member_id", "project_id", "created_at") VALUES (E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, '2019-02-14 08:28:24.677953+00');
INSERT INTO "project_members"("member_id", "project_id", "created_at") VALUES (E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, '2019-02-13 08:28:24.677953+00');

INSERT INTO "irreparabledbs" ("segmentpath", "segmentdetail", "pieces_lost_count", "seg_damaged_unix_sec", "repair_attempt_count") VALUES ('\x49616d5365676d656e746b6579696e666f30', '\x49616d5365676d656e7464657461696c696e666f30', 10, 1550159554, 10);

INSERT INTO "registration_tokens" ("secret", "owner_id", "project_limit", "created_at") VALUES (E'\\070\\127\\144\\013\\332\\344\\102\\376\\306\\056\\303\\130\\106\\132\\321\\276\\321\\274\\170\\264\\054\\333\\221\\116\\154\\221\\335\\070\\220\\146\\344\\216'::bytea, null, 1, '2019-02-14 08:28:24.677953+00');

INSERT INTO "serial_numbers" ("id", "serial_number", "bucket_id", "expires_at") VALUES (1, E'0123456701234567'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014/testbucket'::bytea, '2019-03-06 08:28:24.677953+00');
INSERT INTO "used_serials" ("serial_number_id", "storage_node_id") VALUES (1, E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n');

INSERT INTO "storagenode_bandwidth_rollups" ("storagenode_id", "interval_start", "interval_seconds", "action", "allocated", "settled") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', '2019-03-06 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 3600, 1, 1024, 2024);
INSERT INTO "storagenode_storage_tallies" VALUES (E'\\3510\\323\\225"~\\036<\\342\\330m\\0253Jhr\\246\\233K\\246#\\2303\\351\\256\\275j\\212UM\\362\\207', '2019-02-14 08:16:57.812849+00', 1000);

INSERT INTO "bucket_bandwidth_rollups" ("bucket_name", "project_id", "interval_start", "interval_seconds", "action", "inline", "allocated", "settled") VALUES (E'testbucket'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea,'2019-03-06 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 3600, 1, 1024, 2024, 3024);
INSERT INTO "bucket_storage_tallies" ("bucket_name", "project_id", "interval_start", "inline", "remote", "remote_segments_count", "inline_segments_count", "object_count", "metadata_size") VALUES (E'testbucket'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea,'2019-03-06 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 4024, 5024, 0, 0, 0, 0);
INSERT INTO "bucket_bandwidth_rollups" ("bucket_name", "project_id", "interval_start", "interval_seconds", "action", "inline", "allocated", "settled") VALUES (E'testbucket'::bytea, E'\\170\\160\\157\\370\\274\\366\\113\\364\\272\\235\\301\\243\\321\\102\\321\\136'::bytea,'2019-03-06 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 3600, 1, 1024, 2024, 3024);
INSERT INTO "bucket_storage_tallies" ("bucket_name", "project_id", "interval_start", "inline", "remote", "remote_segments_count", "inline_segments_count", "object_count", "metadata_size") VALUES (E'testbucket'::bytea, E'\\170\\160\\157\\370\\274\\366\\113\\364\\272\\235\\301\\243\\321\\102\\321\\136'::bytea,'2019-03-06 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 4024, 5024, 0, 0, 0, 0);

INSERT INTO "reset_password_tokens" ("secret", "owner_id", "created_at") VALUES (E'\\070\\127\\144\\013\\332\\344\\102\\376\\306\\056\\303\\130\\106\\132\\321\\276\\321\\274\\170\\264\\054\\333\\221\\116\\154\\221\\335\\070\\220\\146\\344\\216'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2019-05-08 08:28:24.677953+00');

INSERT INTO "offers" ("id", "name", "description", "award_credit_in_cents", "invitee_credit_in_cents", "expires_at", "created_at", "status", "type", "award_credit_duration_days", "invitee_credit_duration_days") VALUES (1, 'Default referral offer', 'Is active when no other active referral offer', 300, 600, '2119-03-14 08:28:24.636949+00', '2019-07-14 08:28:24.636949+00', 1, 2, 365, 14);
INSERT INTO "offers" ("id", "name", "description", "award_credit_in_cents", "invitee_credit_in_cents", "expires_at", "created_at", "status", "type", "award_credit_duration_days", "invitee_credit_duration_days") VALUES (2, 'Default free credit offer', 'Is active when no active free credit offer', 0, 300, '2119-03-14 08:28:24.636949+00', '2019-07-14 08:28:24.636949+00', 1, 1, NULL, 14);

INSERT INTO "api_keys" ("id", "project_id", "head", "name", "secret", "partner_id", "created_at") VALUES (E'\\334/\\302;\\225\\355O\\323\\276f\\247\\354/6\\241\\033'::bytea, E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, E'\\111\\142\\147\\304\\132\\375\\070\\163\\270\\160\\251\\370\\126\\063\\351\\037\\257\\071\\143\\375\\351\\320\\253\\232\\220\\260\\075\\173\\306\\307\\115\\136'::bytea, 'key 2', E'\\254\\011\\315\\333\\273\\365\\001\\071\\024\\154\\253\\332\\301\\216\\361\\074\\221\\367\\251\\231\\274\\333\\300\\367\\001\\272\\327\\111\\315\\123\\042\\016'::bytea, NULL, '2019-02-14 08:28:24.267934+00');

INSERT INTO "project_invoice_stamps" ("project_id", "invoice_id", "start_date", "end_date", "created_at") VALUES (E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, E'\\363\\311\\033w\\222\\303,'::bytea, '2019-06-01 08:28:24.267934+00', '2019-06-29 08:28:24.267934+00', '2019-06-01 08:28:24.267934+00');

INSERT INTO "value_attributions" ("project_id", "bucket_name", "partner_id", "last_updated") VALUES (E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, E''::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea,'2019-02-14 08:07:31.028103+00');

INSERT INTO "user_credits" ("id", "user_id", "offer_id", "referred_by", "credits_earned_in_cents", "credits_used_in_cents", "type", "expires_at", "created_at") VALUES (1, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 1, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 200, 0, 'invalid', '2019-10-01 08:28:24.267934+00', '2019-06-01 08:28:24.267934+00');

INSERT INTO "bucket_metainfos" ("id", "project_id", "name", "partner_id", "created_at", "path_cipher", "default_segment_size", "default_encryption_cipher_suite", "default_encryption_block_size", "default_redundancy_algorithm", "default_redundancy_share_size", "default_redundancy_required_shares", "default_redundancy_repair_shares", "default_redundancy_optimal_shares", "default_redundancy_total_shares") VALUES (E'\\334/\\302;\\225\\355O\\323\\276f\\247\\354/6\\241\\033'::bytea, E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, E'testbucketuniquename'::bytea, NULL, '2019-06-14 08:28:24.677953+00', 1, 65536, 1, 8192, 1, 4096, 4, 6, 8, 10);

INSERT INTO "pending_audits" ("node_id", "piece_id", "stripe_index", "share_size", "expected_share_hash", "reverify_count", "path") VALUES (E'\\153\\313\\233\\074\\327\\177\\136\\070\\346\\001'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 5, 1024, E'\\070\\127\\144\\013\\332\\344\\102\\376\\306\\056\\303\\130\\106\\132\\321\\276\\321\\274\\170\\264\\054\\333\\221\\116\\154\\221\\335\\070\\220\\146\\344\\216'::bytea, 1, 'not null');

INSERT INTO "peer_identities" VALUES (E'\\334/\\302;\\225\\355O\\323\\276f\\247\\354/6\\241\\033'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2019-02-14 08:07:31.335028+00');

INSERT INTO "graceful_exit_progress" ("node_id", "bytes_transferred", "pieces_transferred", "pieces_failed", "updated_at") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', 1000000000000000, 0, 0, '2019-09-12 10:07:31.028103+00');
INSERT INTO "graceful_exit_transfer_queue" ("node_id", "path", "piece_num", "durability_ratio", "queued_at", "requested_at", "last_failed_at", "last_failed_code", "failed_count", "finished_at", "order_limit_send_count") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', E'f8419768-5baa-4901-b3ba-62808013ec45/s0/test3/\\240\\243\\223n\\334~b}\\2624)\\250m\\201\\202\\235\\276\\361\\3304\\323\\352\\311\\361\\353;\\326\\311', 8, 1.0, '2019-09-12 10:07:31.028103+00', '2019-09-12 10:07:32.028103+00', null, null, 0, '2019-09-12 10:07:33.028103+00', 0);
INSERT INTO "graceful_exit_transfer_queue" ("node_id", "path", "piece_num", "durability_ratio", "queued_at", "requested_at", "last_failed_at", "last_failed_code", "failed_count", "finished_at", "order_limit_send_count") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', E'f8419768-5baa-4901-b3ba-62808013ec45/s0/test3/\\240\\243\\223n\\334~b}\\2624)\\250m\\201\\202\\235\\276\\361\\3304\\323\\352\\311\\361\\353;\\326\\312', 8, 1.0, '2019-09-12 10:07:31.028103+00', '2019-09-12 10:07:32.028103+00', null, null, 0, '2019-09-12 10:07:33.028103+00', 0);

INSERT INTO "stripe_customers" ("user_id", "customer_id", "created_at") VALUES (E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 'stripe_id', '2019-06-01 08:28:24.267934+00');

INSERT INTO "graceful_exit_transfer_queue" ("node_id", "path", "piece_num", "durability_ratio", "queued_at", "requested_at", "last_failed_at", "last_failed_code", "failed_count", "finished_at", "order_limit_send_count") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', E'f8419768-5baa-4901-b3ba-62808013ec45/s0/test3/\\240\\243\\223n\\334~b}\\2624)\\250m\\201\\202\\235\\276\\361\\3304\\323\\352\\311\\361\\353;\\326\\311', 9, 1.0, '2019-09-12 10:07:31.028103+00', '2019-09-12 10:07:32.028103+00', null, null, 0, '2019-09-12 10:07:33.028103+00', 0);
INSERT INTO "graceful_exit_transfer_queue" ("node_id", "path", "piece_num", "durability_ratio", "queued_at", "requested_at", "last_failed_at", "last_failed_code", "failed_count", "finished_at", "order_limit_send_count") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', E'f8419768-5baa-4901-b3ba-62808013ec45/s0/test3/\\240\\243\\223n\\334~b}\\2624)\\250m\\201\\202\\235\\276\\361\\3304\\323\\352\\311\\361\\353;\\326\\312', 9, 1.0, '2019-09-12 10:07:31.028103+00', '2019-09-12 10:07:32.028103+00', null, null, 0, '2019-09-12 10:07:33.028103+00', 0);

INSERT INTO "stripecoinpayments_invoice_project_records"("id", "project_id", "storage", "egress", "objects", "period_start", "period_end", "state", "created_at") VALUES (E'\\022\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, E'\\021\\217/\\014\\376!K\\023\\276\\031\\311}m\\236\\205\\300'::bytea, 0, 0, 0, '2019-06-01 08:28:24.267934+00', '2019-06-01 08:28:24.267934+00', 0, '2019-06-01 08:28:24.267934+00');

INSERT INTO "graceful_exit_transfer_queue" ("node_id", "path", "piece_num", "root_piece_id", "durability_ratio", "queued_at", "requested_at", "last_failed_at", "last_failed_code", "failed_count", "finished_at", "order_limit_send_count") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016', E'f8419768-5baa-4901-b3ba-62808013ec45/s0/test3/\\240\\243\\223n\\334~b}\\2624)\\250m\\201\\202\\235\\276\\361\\3304\\323\\352\\311\\361\\353;\\326\\311', 10, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 1.0, '2019-09-12 10:07:31.028103+00', '2019-09-12 10:07:32.028103+00', null, null, 0, '2019-09-12 10:07:33.028103+00', 0);

INSERT INTO "stripecoinpayments_tx_conversion_rates" ("tx_id", "rate", "created_at") VALUES ('tx_id', E'\\363\\311\\033w\\222\\303Ci,'::bytea, '2019-06-01 08:28:24.267934+00');

INSERT INTO "coinpayments_transactions" ("id", "user_id", "address", "amount", "received", "status", "key", "timeout", "created_at") VALUES ('tx_id', E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 'address', E'\\363\\311\\033w'::bytea, E'\\363\\311\\033w'::bytea, 1, 'key', 60, '2019-06-01 08:28:24.267934+00');

INSERT INTO "nodes_offline_times" ("node_id", "tracked_at", "seconds") VALUES (E'\\153\\313\\233\\074\\327\\177\\136\\070\\346\\001'::bytea, '2019-06-01 09:28:24.267934+00', 3600);
INSERT INTO "nodes_offline_times" ("node_id", "tracked_at", "seconds") VALUES (E'\\153\\313\\233\\074\\327\\177\\136\\070\\346\\001'::bytea, '2017-06-01 09:28:24.267934+00', 100);
INSERT INTO "nodes_offline_times" ("node_id", "tracked_at", "seconds") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n'::bytea, '2019-06-01 09:28:24.267934+00', 3600);

INSERT INTO "storagenode_bandwidth_rollups" ("storagenode_id", "interval_start", "interval_seconds", "action", "settled") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', '2020-01-11 08:00:00.000000' AT TIME ZONE current_setting('TIMEZONE'), 3600, 1, 2024);

INSERT INTO "coupons" ("id", "project_id", "user_id", "amount", "description", "type", "status", "duration", "created_at") VALUES (E'\\362\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 50, 'description', 0, 0, 2, '2019-06-01 08:28:24.267934+00');
INSERT INTO "coupon_usages" ("coupon_id", "amount", "status", "period") VALUES (E'\\362\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, 22, 0, '2019-06-01 09:28:24.267934+00');

INSERT INTO "reported_serials" ("expires_at", "storage_node_id", "bucket_id", "action", "serial_number", "settled", "observed_at") VALUES ('2020-01-11 08:00:00.000000+00', E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014/testbucket'::bytea, 1, E'0123456701234567'::bytea, 100, '2020-01-11 08:00:00.000000+00');

INSERT INTO "stripecoinpayments_apply_balance_intents" ("tx_id", "state", "created_at") VALUES ('tx_id', 0, '2019-06-01 08:28:24.267934+00');

INSERT INTO "projects"("id", "name", "description", "usage_limit", "rate_limit", "partner_id", "owner_id", "created_at") VALUES (E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\347'::bytea, 'projName1', 'Test project 1', 0, 2000000, NULL, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, '2020-01-15 08:28:24.636949+00');

INSERT INTO "credits" ("user_id", "transaction_id", "amount", "created_at") VALUES (E'\\362\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014'::bytea, 'transactionID', 10, '2019-06-01 08:28:24.267934+00');
INSERT INTO "credits_spendings" ("id", "user_id", "project_id", "amount", "status", "created_at") VALUES (E'\\362\\342\\363\\371>+F\\256\\263\\300\\275|\\342N\\347\\014'::bytea, E'\\153\\313\\233\\074\\327\\177\\136\\070\\346\\001'::bytea, E'\\363\\311\\033w\\222\\303Ci\\265\\343U\\303\\312\\204",'::bytea, 5, 0, '2019-06-01 09:28:24.267934+00');

INSERT INTO "pending_serial_queue" ("storage_node_id", "bucket_id", "serial_number", "action", "settled", "expires_at") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\014/testbucket'::bytea, E'5123456701234567'::bytea, 1, 100, '2020-01-11 08:00:00.000000+00');

INSERT INTO "consumed_serials" ("storage_node_id", "serial_number", "expires_at") VALUES (E'\\006\\223\\250R\\221\\005\\365\\377v>0\\266\\365\\216\\255?\\347\\244\\371?2\\264\\262\\230\\007<\\001\\262\\263\\237\\247n', E'1234567012345678'::bytea, '2020-01-12 08:00:00.000000+00');

INSERT INTO "injuredsegments" ("path", "data", "num_healthy_pieces") VALUES ('0', '\x0a0130120100', 52);
INSERT INTO "injuredsegments" ("path", "data", "num_healthy_pieces") VALUES ('here''s/a/great/path', '\x0a136865726527732f612f67726561742f70617468120a0102030405060708090a', 30);
INSERT INTO "injuredsegments" ("path", "data", "num_healthy_pieces") VALUES ('yet/another/cool/path', '\x0a157965742f616e6f746865722f636f6f6c2f70617468120a0102030405060708090a', 51);
INSERT INTO "injuredsegments" ("path", "data", "num_healthy_pieces") VALUES ('/this/is/a/new/path', '\x0a23736f2f6d616e792f69636f6e69632f70617468732f746f2f63686f6f73652f66726f6d120a0102030405060708090a', 40);

INSERT INTO "nodes"("id", "address", "last_net", "last_ip_port", "protocol", "type", "email", "wallet", "free_disk", "piece_count", "major", "minor", "patch", "hash", "timestamp", "release","latency_90", "audit_success_count", "total_audit_count", "uptime_success_count", "total_uptime_count", "created_at", "updated_at", "last_contact_success", "last_contact_failure", "contained", "disqualified", "suspended", "audit_reputation_alpha", "audit_reputation_beta", "unknown_audit_reputation_alpha", "unknown_audit_reputation_beta", "uptime_reputation_alpha", "uptime_reputation_beta", "exit_success") VALUES (E'\\154\\313\\233\\074\\327\\177\\136\\070\\346\\002', '127.0.0.1:55516', '127.0.0.0', '127.0.0.1:55516', 0, 4, '', '', -1, 0, 0, 1, 0, '', 'epoch', false, 0, 0, 5, 0, 5, '2019-02-14 08:07:31.028103+00', '2019-02-14 08:07:31.108963+00', 'epoch', 'epoch', false, NULL, '2019-02-14 08:07:31.028103+00', 50, 0, 75, 25, 100, 5, false);

UPDATE "nodes" SET vetted_at='2020-03-18 12:00:00.000000+00' where id = E'\\363\\342\\363\\371>+F\\256\\263\\300\\273|\\342N\\347\\016';

-- NEW DATA --
