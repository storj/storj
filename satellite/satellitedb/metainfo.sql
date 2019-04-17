CREATE TYPE encryption_parameters AS (
    -- total 5 bytes
    ciphersuite BYTE NOT NULL;
    block_size  INT4 NOT NULL;
);

CREATE TYPE redundancy_scheme AS (
    -- total 9 bytes
    algorithm   BYTE   NOT NULL;
    share_size  INT4   NOT NULL;
    required    INT2   NOT NULL;
    repair      INT2   NOT NULL;
    optimal     INT2   NOT NULL;
    total       INT2   NOT NULL;
);

CREATE TABLE buckets (
    bucket_id   UUID; -- 16 bytes, should we use here a serial of 8 bytes?
    project_id  UUID; -- 16 bytes

    bucket_name BYTEA NOT NULL;     -- ? bytes
    created_at  TIMESTAMP NOT NULL; -- 8 bytes
    modified_at TIMESTAMP NOT NULL; -- 8 bytes

    path_encryption_algorithm BYTE NOT NULL; -- 1 byte

    default_segment_size INT4 NOT NULL;

    default_encryption   encryption_parameters NOT NULL; -- 5 bytes
    default_redundancy   redundancy_scheme     NOT NULL; -- 9 bytes

    PRIMARY KEY (bucket_id, project_id);
)

CREATE TYPE object_status AS ENUM ('partial', 'committed', 'deleting');

CREATE TABLE objects (
    bucket_id      UUID  NOT NULL REFERENCES buckets(bucket_id); -- 16 bytes (or 8 bytes when serial)
    encrypted_path BYTEA NOT NULL; -- ?  bytes
    stream_id      UUID  NOT NULL; -- 16 bytes (or should this be a serial of 8 bytes)

    status         object_status  NOT NULL DEFAULT 'partial'; -- 1 byte
    version        INT4           NOT NULL DEFAULT 0;         -- 4 bytes

    created_at          TIMESTAMP NOT NULL; -- 8 bytes
    expires_at          TIMESTAMP NOT NULL; -- 8 bytes
    encrypted_metadata  BYTEA     NOT NULL; -- ? bytes

    data_checksum      INT8       NOT NULL DEFAULT -1; -- 8 bytes, checksum of checksums (do we need this)
    total_size         INT8       NOT NULL DEFAULT -1; -- 8 bytes
    fixed_segment_size INT4       NOT NULL DEFAULT -1; -- 4 bytes
    segment_count      INT4       NOT NULL DEFAULT -1; -- 8 bytes (do we need this or can we use segments table as source of truth)

    encryption encryption_parameters NOT NULL; -- 5 bytes (should this be a 4 byte reference to a table)
    redundancy redundancy_scheme     NOT NULL; -- 9 bytes (should this be a 4 byte reference to a table)

    PRIMARY KEY (bucket_id, encrypted_path, version);
)

CREATE TABLE segments (
    stream_id            UUID   NOT NULL; -- 16 bytes (or should this be a serial of 8 bytes)

    segment_index        INT4   DEFAULT NULL;  -- 4 bytes
    segment_upload_index INT8   NOT NULL;      -- 8 bytes (is there a way to have this only temporarily)

    root_piece_id        BYTEA  NOT NULL; -- 32 bytes
    encrypted_key_nonce  BYTEA  NOT NULL; -- 32 bytes
    encrypted_key        BYTEA  NOT NULL; -- 32 bytes

    data_checksum        INT8   NOT NULL; -- 8 bytes (do we need this, is this encrypted_data_checksum or cleardata_checksum)
    size                 INT4   NOT NULL DEFAULT -1; -- 4 bytes
    inline_data_or_nodes BYTEA  NOT NULL; -- 100 * 32 bytes (or 100 * 8 bytes with node link optimization)

    PRIMARY KEY (stream_id, segment_index);
)

----------------------------------------------------------------------------------------------------------------------------------------------------------------
-- alternate implementation of segments
----------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE TABLE committed_segments (
    stream_id            UUID   NOT NULL; -- 16 bytes (or should this be a serial of 8 bytes)

    segment_index        INT4   NOT NULL; -- 4 bytes

    root_piece_id        BYTEA  NOT NULL; -- 32 bytes
    encrypted_key_nonce  BYTEA  NOT NULL; -- 32 bytes
    encrypted_key        BYTEA  NOT NULL; -- 32 bytes

    data_checksum        INT8   NOT NULL; -- 8 bytes (do we need this, is this encrypted_data_checksum or cleardata_checksum)
    size                 INT4   NOT NULL; -- 4 bytes
    inline_data_or_nodes BYTEA  NOT NULL; -- 100 * 32 bytes (or 100 * 8 bytes with node link optimization)

    PRIMARY KEY (stream_id, segment_index);
)

CREATE TABLE partial_segments (
    stream_id            UUID   NOT NULL; -- 16 bytes (or should this be a serial of 8 bytes)

    segment_upload_index INT8   NOT NULL; -- 8 bytes

    root_piece_id        BYTEA  NOT NULL; -- 32 bytes
    encrypted_key_nonce  BYTEA  NOT NULL; -- 32 bytes
    encrypted_key        BYTEA  NOT NULL; -- 32 bytes

    data_checksum        INT8   NOT NULL; -- 8 bytes (do we need this, is this encrypted_data_checksum or cleardata_checksum)
    size                 INT4   NOT NULL; -- 4 bytes
    inline_data_or_nodes BYTEA  NOT NULL; -- 100 * 32 bytes (or 100 * 8 bytes with node link optimization)

    PRIMARY KEY (stream_id, segment_upload_index);
)