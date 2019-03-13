// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

type orders struct {
	*infodb
}

// Orders returns database for storing orders
func (db *infodb) Orders() orders { return orders{db} }

// OrderInfo contains full information about an order.
// TODO: move to a better location.
type OrderInfo struct {
	Limit  *pb.OrderLimit2
	Order  *pb.Order2
	Uplink *identity.PeerIdentity
}

// Enqueue inserts order to the unsent list
func (db *orders) Enqueue(ctx context.Context, info *OrderInfo) error {
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
func (db *orders) ListUnsent(ctx context.Context, limit int) (_ []*OrderInfo, err error) {
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

	var infos []*OrderInfo
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte
		var uplinkPEM []byte

		err := rows.Scan(&limitSerialized, &orderSerialized, &uplinkPEM)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		var info OrderInfo
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

		info.Uplink, err = identity.PeerIdentityFromPEM(uplinkPEM)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrInfo.Wrap(rows.Err())
}
