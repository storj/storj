-- table for keeping serials that need to be verified against
CREATE TABLE used_serial_ (
    satellite_id  BLOB NOT NULL,
    serial_number BLOB NOT NULL,
    expiration    TIMESTAMP NOT NULL
);
-- primary key on satellite id and serial number
CREATE UNIQUE INDEX pk_used_serial_ ON used_serial_(satellite_id, serial_number);
-- expiration index to allow fast deletion
CREATE INDEX idx_used_serial_ ON used_serial_(expiration);

-- certificate table for storing uplink/satellite certificates
CREATE TABLE certificate (
    cert_id       INTEGER
);

-- table for storing piece meta info
CREATE TABLE pieceinfo_ (
    satellite_id     BLOB      NOT NULL,
    piece_id         BLOB      NOT NULL,
    piece_size       BIGINT    NOT NULL,
    piece_expiration TIMESTAMP,

    order_limit       BLOB    NOT NULL,
    uplink_piece_hash BLOB    NOT NULL,
    uplink_cert_id    INTEGER NOT NULL,

    deletion_failed_at TIMESTAMP,
    piece_creation TIMESTAMP NOT NULL,

    FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
);
-- primary key by satellite id and piece id
CREATE UNIQUE INDEX pk_pieceinfo_ ON pieceinfo_(satellite_id, piece_id);
-- fast queries for expiration for pieces that have one
CREATE INDEX idx_pieceinfo__expiration ON pieceinfo_(piece_expiration) WHERE piece_expiration IS NOT NULL;

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
CREATE TABLE order_archive_ (
    satellite_id  BLOB NOT NULL,
    serial_number BLOB NOT NULL,

    order_limit_serialized BLOB NOT NULL,
    order_serialized       BLOB NOT NULL,

    uplink_cert_id INTEGER NOT NULL,

    status      INTEGER   NOT NULL,
    archived_at TIMESTAMP NOT NULL,

    FOREIGN KEY(uplink_cert_id) REFERENCES certificate(cert_id)
);

-- table for storing vouchers
CREATE TABLE vouchers (
    satellite_id BLOB PRIMARY KEY NOT NULL,
    voucher_serialized BLOB NOT NULL,
    expiration TIMESTAMP NOT NULL
);

CREATE TABLE bandwidth_usage_rollups (
    interval_start	TIMESTAMP NOT NULL,
    satellite_id  	BLOB    NOT NULL,
    action        	INTEGER NOT NULL,
    amount        	BIGINT  NOT NULL,
    PRIMARY KEY ( interval_start, satellite_id, action )
);

INSERT INTO unsent_order VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',X'1eddef484b4c03f01332279032796972',X'0a101eddef484b4c03f0133227903279697212202b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf410001a201968996e7ef170a402fdfd88b6753df792c063c07c555905ffac9cd3cbd1c00022200ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac30002a20d00cf14f3c68b56321ace04902dec0484eb6f9098b22b31c6b3f82db249f191630643802420c08dfeb88e50510a8c1a5b9034a0c08dfeb88e50510a8c1a5b9035246304402204df59dc6f5d1bb7217105efbc9b3604d19189af37a81efbf16258e5d7db5549e02203bb4ead16e6e7f10f658558c22b59c3339911841e8dbaae6e2dea821f7326894',X'0a101eddef484b4c03f0133227903279697210321a47304502206d4c106ddec88140414bac5979c95bdea7de2e0ecc5be766e08f7d5ea36641a7022100e932ff858f15885ffa52d07e260c2c25d3861810ea6157956c1793ad0c906284','2019-04-01 16:01:35.9254586+00:00',1);

INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',0,0,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',0,0,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',1,1,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',2,2,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',2,2,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',3,3,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',3,3,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',4,4,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',4,4,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',5,5,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',5,5,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',6,6,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',6,6,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',1,1,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',2,2,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',2,2,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',3,3,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',3,3,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',4,4,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',4,4,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',5,5,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',5,5,'2019-04-01 20:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',6,6,'2019-04-01 18:51:24.1074772+00:00');
INSERT INTO bandwidth_usage VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',6,6,'2019-04-01 20:51:24.1074772+00:00');

INSERT INTO vouchers VALUES(X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000', X'd5e757fd8d207d1c46583fb58330f803dc961b71147308ff75ff1e72a0df6b0b', '2019-07-04 00:00:00.000000+00:00');

INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',0,0);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',0,0);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',1,1);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',1,1);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',2,2);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',2,2);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',3,3);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',3,3);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',4,4);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',4,4);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',5,5);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',5,5);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 18:00:00+00:00',X'0ed28abb2813e184a1e98b0f6605c4911ea468c7e8433eb583e0fca7ceac3000',6,6);
INSERT INTO bandwidth_usage_rollups VALUES('2019-07-12 20:00:00+00:00',X'2b3a5863a41f25408a8f5348839d7a1361dbd886d75786bb139a8ca0bdf41000',6,6);

-- NEW DATA --
