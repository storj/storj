CREATE TABLE IF NOT EXISTS segments
(
    stream_id           BYTES( MAX) NOT NULL,
    position            INT64 NOT NULL,
    created_at          TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
    repaired_at         TIMESTAMP,
    expires_at          TIMESTAMP,
    root_piece_id       BYTES( MAX) NOT NULL,
    encrypted_key_nonce BYTES( MAX) NOT NULL,
    encrypted_key       BYTES( MAX) NOT NULL,
    encrypted_size      INT64 NOT NULL,
    encrypted_etag      BYTES( MAX),
    plain_offset        INT64 NOT NULL,
    plain_size          INT64 NOT NULL,
    redundancy          INT64 NOT NULL,
    inline_data         BYTES( MAX),
    remote_alias_pieces BYTES(MAX),
    placement           INT64 NOT NULL DEFAULT (1),
    ) PRIMARY KEY(stream_id, position);

CREATE TABLE IF NOT EXISTS
    objects
(
    project_id                       BYTES(MAX) NOT NULL,
    bucket_name                      STRING(MAX) NOT NULL,
    object_key                       BYTES(MAX) NOT NULL,
    version                          INT64     NOT NULL,
    stream_id                        BYTES( MAX) NOT NULL,
    created_at                       TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP()),
    expires_at                       TIMESTAMP,
    status                           INT64     NOT NULL DEFAULT (1),
    segment_count                    INT64     NOT NULL DEFAULT (0),
    encrypted_metadata_nonce         BYTES( MAX),
    encrypted_metadata               BYTES( MAX),
    encrypted_metadata_encrypted_key BYTES( MAX),
    total_plain_size                 INT64     NOT NULL DEFAULT (0),
    total_encrypted_size             INT64     NOT NULL DEFAULT (0),
    fixed_segment_size               INT64     NOT NULL DEFAULT (0),
    encryption                       INT64     NOT NULL DEFAULT (0),
    zombie_deletion_deadline         TIMESTAMP,
    retention_mode                   INT64     NOT NULL DEFAULT (0),
    retain_until                     TIMESTAMP,
    ) PRIMARY KEY
(project_id,
 bucket_name,
 object_key,
 version);

CREATE TABLE IF NOT EXISTS
    node_aliases
(
    node_id     BYTES(MAX) NOT NULL,
    node_alias  INT64      NOT NULL,
    ) PRIMARY KEY
(node_id);
CREATE UNIQUE INDEX IF NOT EXISTS node_aliases_node_alias_key ON node_aliases(node_alias);