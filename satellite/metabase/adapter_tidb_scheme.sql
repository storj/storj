-- This schema targets TiDB v8.5+ (LTS) and assumes the cluster is started with
-- `max-index-length = 12288` in the TiDB configuration file (the default 3072
-- is too small for the objects PK because object_key alone can be up to
-- MaxEncryptedObjectKeyLength = 4000 bytes). Without that setting, TiDB
-- rejects this CREATE TABLE with "Specified key was too long".
CREATE TABLE IF NOT EXISTS objects (
    project_id                       VARBINARY(16)    NOT NULL,
    bucket_name                      VARBINARY(64)    NOT NULL,
    object_key                       VARBINARY(12180) NOT NULL,
    version                          BIGINT           NOT NULL,
    stream_id                        VARBINARY(16)    NOT NULL,
    created_at                       DATETIME(6)      NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at                       DATETIME(6)      NULL,
    status                           TINYINT UNSIGNED NOT NULL DEFAULT 1,
    segment_count                    INT              NOT NULL DEFAULT 0,
    encrypted_metadata_nonce         BLOB             NULL,
    encrypted_metadata               MEDIUMBLOB       NULL,
    encrypted_metadata_encrypted_key BLOB             NULL,
    total_plain_size                 BIGINT           NOT NULL DEFAULT 0,
    total_encrypted_size             BIGINT           NOT NULL DEFAULT 0,
    fixed_segment_size               INT              NOT NULL DEFAULT 0,
    encryption                       BIGINT           NOT NULL DEFAULT 0,
    zombie_deletion_deadline         DATETIME(6)      NULL,
    retention_mode                   TINYINT UNSIGNED NULL,
    retain_until                     DATETIME(6)      NULL,
    product_id                       INT              NULL,
    encrypted_etag                   MEDIUMBLOB       NULL,
    checksum                         MEDIUMBLOB       NULL,
    PRIMARY KEY (project_id, bucket_name, object_key, version) /*T![clustered_index] CLUSTERED */
);

CREATE TABLE IF NOT EXISTS segments (
    stream_id           VARBINARY(16)  NOT NULL,
    position            BIGINT         NOT NULL,
    created_at          DATETIME(6)    NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    repaired_at         DATETIME(6)    NULL,
    expires_at          DATETIME(6)    NULL,
    root_piece_id       VARBINARY(32)  NOT NULL,
    encrypted_key_nonce BLOB           NOT NULL,
    encrypted_key       BLOB           NOT NULL,
    encrypted_size      INT            NOT NULL,
    encrypted_etag      MEDIUMBLOB     NULL,
    encrypted_checksum  MEDIUMBLOB     NULL,
    plain_offset        BIGINT         NOT NULL,
    plain_size          INT            NOT NULL,
    redundancy          BIGINT         NOT NULL DEFAULT 0,
    inline_data         MEDIUMBLOB     NULL,
    remote_alias_pieces MEDIUMBLOB     NULL,
    placement           INT            NULL,
    PRIMARY KEY (stream_id, position) /*T![clustered_index] CLUSTERED */
);

CREATE TABLE IF NOT EXISTS node_aliases (
    node_id    VARBINARY(32) NOT NULL,
    node_alias INT           NOT NULL AUTO_INCREMENT,
    PRIMARY KEY (node_id) /*T![clustered_index] CLUSTERED */,
    UNIQUE KEY node_aliases_node_alias_key (node_alias)
);
