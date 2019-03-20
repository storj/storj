// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/orders"
)

type ordersdb struct{ *infodb }

// Orders returns database for storing orders
func (db *DB) Orders() orders.DB { return db.info.Orders() }

// Orders returns database for storing orders
func (db *infodb) Orders() orders.DB { return &ordersdb{db} }

// Enqueue inserts order to the unsent list
func (db *ordersdb) Enqueue(ctx context.Context, info *orders.Info) error {
	certdb := db.CertDB()

	uplinkCertID, err := certdb.Include(ctx, info.Uplink)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	limitSerialized, err := proto.Marshal(info.Limit)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	orderSerialized, err := proto.Marshal(info.Order)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	expirationTime, err := ptypes.Timestamp(info.Limit.OrderExpiration)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	defer db.locked()()

	_, err = db.db.Exec(`
		INSERT INTO unsent_order(
			satellite_id, serial_number,
			order_limit_serialized, order_serialized, order_limit_expiration,
			uplink_cert_id
		) VALUES (?,?, ?,?,?, ?)
	`, info.Limit.SatelliteId, info.Limit.SerialNumber, limitSerialized, orderSerialized, expirationTime, uplinkCertID)

	return ErrInfo.Wrap(err)
}

// ListUnsent returns orders that haven't been sent yet.
func (db *ordersdb) ListUnsent(ctx context.Context, limit int) (_ []*orders.Info, err error) {
	defer db.locked()()

	rows, err := db.db.Query(`
		SELECT order_limit_serialized, order_serialized, certificate.peer_identity
		FROM unsent_order
		INNER JOIN certificate on unsent_order.uplink_cert_id = certificate.cert_id
		LIMIT ?
	`, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var infos []*orders.Info
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte
		var uplinkIdentity []byte

		err := rows.Scan(&limitSerialized, &orderSerialized, &uplinkIdentity)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		var info orders.Info
		info.Limit = &pb.OrderLimit2{}
		info.Order = &pb.Order2{}

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		info.Uplink, err = decodePeerIdentity(uplinkIdentity)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrInfo.Wrap(rows.Err())
}

// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
func (db *ordersdb) ListUnsentBySatellite(ctx context.Context) (map[storj.NodeID][]*orders.Info, error) {
	// TODO: add some limiting

	rows, err := db.db.Query(`
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	infos := map[storj.NodeID][]*orders.Info{}
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		var info orders.Info
		info.Limit = &pb.OrderLimit2{}
		info.Order = &pb.Order2{}

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		infos[info.Limit.SatelliteId] = append(infos[info.Limit.SatelliteId], &info)
	}

	return infos, ErrInfo.Wrap(rows.Err())
}

// Archive marks order as being handled.
func (db *ordersdb) Archive(ctx context.Context, satellite storj.NodeID, serial storj.SerialNumber, status orders.Status) error {
	return ErrInfo.New("unhandled")
}
