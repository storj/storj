// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"storj.io/storj/internal/migrate"
)

func (db *DB) Migration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					// certificate table for storing uplink/satellite certificates
					`CREATE TABLE certificate (
						certid            SERIAL PRIMARY KEY,
						certificate_pkix  BLOB UNIQUE
					)`,

					// table for storing piece meta info
					`CREATE TABLE pieceinfo (
						satellite_id BLOB,
						id           BLOB,
						size         BIGINT,
						expiration   TIMESTAMP without time zone, -- date when it can be deleted

						uplink_hash   BLOB, -- serialized pb.PieceHash signed by uplink
						uplink_certid SERIAL,

						FOREIGN KEY(uplink_certid) REFERENCES certificate(certid)
					)`,
					// primary key by satellite id and piece id
					`ALTER TABLE pieceinfo
						ADD CONSTRAINT pk_pieceinfo ON pieceinfo(satellite_id, id)`,

					// table for storing order information
					`CREATE TABLE orderinfo (
						satellite     BLOB,
						action        INTEGER,
						amount        BIGINT,
						created_at    TIMESTAMP without time zone
					)`,

					// table for storing all unsent orders
					`CREATE TABLE orders_unsent (
						satellite     BLOB,

						order_limit   BLOB, -- serialized pb.OrderLimit
						order         BLOB, -- serialized pb.Order
						order_limit_expiration TIMESTAMP without time zone, -- when is the deadline for sending it

						uplink_certid SERIAL,

						FOREIGN KEY(uplink_certid) REFERENCES certificate(certid)
					)`,

					// table for storing all rejected orders
					`CREATE TABLE orders_rejected (
						satellite     BLOB,

						order_limit   BLOB, -- serialized pb.OrderLimit
						order         BLOB, -- serialized pb.Order

						uplink_certid SERIAL,

						rejected_at TIMESTAMP without time zone, -- when was it rejected

						FOREIGN KEY(uplink_certid) REFERENCES certificate(certid)
					)`,

					// table for keeping serials that need to be verified against
					`CREATE TABLE used_serials (
						satellite_id  BLOB,
						serial_number BLOB,
						expiration    TIMESTAMP without time zone
					)`,
					// primary key on satellite id and serial number
					`ALTER TABLE used_serials 
						ADD CONSTRAINT pk_used_serials ON used_serials(satellite_id, serial_number)`,
					// expiration index to allow fast deletion
					`CREATE INDEX idx_used_serials ON used_serials(expiration)`,
				},
			},
		},
	}
}
