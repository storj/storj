CREATE TABLE IF NOT EXISTS segments
(
    stream_id           BYTES(16) NOT NULL,
    position            INT64 NOT NULL,
    created_at          TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
    repaired_at         TIMESTAMP,
    expires_at          TIMESTAMP,
    root_piece_id       BYTES(32) NOT NULL,
    encrypted_key_nonce BYTES(MAX) NOT NULL,
    encrypted_key       BYTES(MAX) NOT NULL,
    encrypted_size      INT64 NOT NULL,
    encrypted_etag      BYTES(MAX),
    plain_offset        INT64 NOT NULL,
    plain_size          INT64 NOT NULL,
    redundancy          INT64 NOT NULL DEFAULT (0),
    inline_data         BYTES(MAX),
    remote_alias_pieces BYTES(MAX),
    placement           INT64,
) PRIMARY KEY(stream_id, position);

CREATE TABLE IF NOT EXISTS objects
(
    project_id                       BYTES(16) NOT NULL,
    bucket_name                      STRING(MAX) NOT NULL,
    object_key                       BYTES(MAX) NOT NULL,
    version                          INT64     NOT NULL,
    stream_id                        BYTES(16) NOT NULL,
    created_at                       TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
    expires_at                       TIMESTAMP,
    status                           INT64     NOT NULL DEFAULT (1),
    segment_count                    INT64     NOT NULL DEFAULT (0),
    encrypted_metadata_nonce         BYTES(MAX),
    encrypted_metadata               BYTES(MAX),
    encrypted_metadata_encrypted_key BYTES(MAX),
    total_plain_size                 INT64     NOT NULL DEFAULT (0),
    total_encrypted_size             INT64     NOT NULL DEFAULT (0),
    fixed_segment_size               INT64     NOT NULL DEFAULT (0),
    encryption                       INT64     NOT NULL DEFAULT (0),
    zombie_deletion_deadline         TIMESTAMP,
    retention_mode                   INT64,
    retain_until                     TIMESTAMP,
    product_id                       INT64,
    encrypted_etag                   BYTES(MAX),
) PRIMARY KEY (project_id, bucket_name, object_key, version);

CREATE TABLE IF NOT EXISTS node_aliases
(
    node_id     BYTES(32)  NOT NULL,
    node_alias  INT64      NOT NULL,
) PRIMARY KEY (node_id);

CREATE UNIQUE INDEX IF NOT EXISTS node_aliases_node_alias_key ON node_aliases(node_alias);

CREATE CHANGE STREAM bucket_eventing FOR objects (stream_id, status, total_plain_size) OPTIONS ( value_capture_type = 'NEW_ROW_AND_OLD_VALUES', exclude_ttl_deletes = TRUE );

CREATE TABLE IF NOT EXISTS bucket_eventing_metadata
(
    partition_token STRING(MAX) NOT NULL,
    parent_tokens   ARRAY<STRING(MAX)>,
    start_timestamp TIMESTAMP NOT NULL,
    state           INT64     NOT NULL DEFAULT (0),
    watermark       TIMESTAMP NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
    scheduled_at    TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
    running_at      TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
    finished_at     TIMESTAMP OPTIONS (allow_commit_timestamp = TRUE),
)
PRIMARY KEY (partition_token), ROW DELETION POLICY (OLDER_THAN(finished_at, INTERVAL 7 DAY));

CREATE INDEX IF NOT EXISTS bucket_eventing_metadata_state ON bucket_eventing_metadata(state);