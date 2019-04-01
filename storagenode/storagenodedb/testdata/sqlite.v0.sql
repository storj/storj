-- table for keeping serials that need to be verified against
CREATE TABLE used_serial (
    satellite_id  BLOB NOT NULL,
    serial_number BLOB NOT NULL,
    expiration    TIMESTAMP NOT NULL
);
-- primary key on satellite id and serial number
CREATE UNIQUE INDEX pk_used_serial ON used_serial(satellite_id, serial_number);
-- expiration index to allow fast deletion
CREATE INDEX idx_used_serial ON used_serial(expiration);

-- certificate table for storing uplink/satellite certificates
CREATE TABLE certificate (
    cert_id       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    node_id       BLOB        NOT NULL,
    peer_identity BLOB UNIQUE NOT NULL
);

-- table for storing piece meta info
CREATE TABLE pieceinfo (
    satellite_id     BLOB      NOT NULL,
    piece_id         BLOB      NOT NULL,
    piece_size       BIGINT    NOT NULL,
    piece_expiration TIMESTAMP,

    uplink_piece_hash BLOB    NOT NULL,
    uplink_cert_id    INTEGER NOT NULL,

    FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
);
-- primary key by satellite id and piece id
CREATE UNIQUE INDEX pk_pieceinfo ON pieceinfo(satellite_id, piece_id);

-- table for storing bandwidth usage
CREATE TABLE bandwidth_usage (
    satellite_id  BLOB    NOT NULL,
    action        INTEGER NOT NULL,
    amount        BIGINT  NOT NULL,
    created_at    TIMESTAMP NOT NULL
);
CREATE INDEX idx_bandwidth_usage_satellite ON bandwidth_usage(satellite_id);
CREATE INDEX idx_bandwidth_usage_created   ON bandwidth_usage(created_at);

-- table for storing all unsent orders
CREATE TABLE unsent_order (
    satellite_id  BLOB NOT NULL,
    serial_number BLOB NOT NULL,

    order_limit_serialized BLOB      NOT NULL,
    order_serialized       BLOB      NOT NULL,
    order_limit_expiration TIMESTAMP NOT NULL,

    uplink_cert_id INTEGER NOT NULL,

    FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
);
CREATE UNIQUE INDEX idx_orders ON unsent_order(satellite_id, serial_number);

-- table for storing all sent orders
CREATE TABLE order_archive (
    satellite_id  BLOB NOT NULL,
    serial_number BLOB NOT NULL,
    
    order_limit_serialized BLOB NOT NULL,
    order_serialized       BLOB NOT NULL,
    
    uplink_cert_id INTEGER NOT NULL,
    
    status      INTEGER   NOT NULL,
    archived_at TIMESTAMP NOT NULL,
    
    FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
);
CREATE INDEX idx_order_archive_satellite ON order_archive(satellite_id);
CREATE INDEX idx_order_archive_status ON order_archive(status);

-- NEW DATA --

