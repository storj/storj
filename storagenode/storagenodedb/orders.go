// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/orders"
)

type ordersdb struct{ *InfoDB }

// Orders returns database for storing orders
func (db *DB) Orders() orders.DB { return db.info.Orders() }

// Orders returns database for storing orders
func (db *InfoDB) Orders() orders.DB { return &ordersdb{db} }

// Enqueue inserts order to the unsent list
func (db *ordersdb) Enqueue(ctx context.Context, info *orders.Info) (err error) {
	defer mon.Task()(&ctx)(&err)

	limitSerialized, err := proto.Marshal(info.Limit)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	orderSerialized, err := proto.Marshal(info.Order)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	// TODO: remove uplink_cert_id
	_, err = db.db.Exec(`
		INSERT INTO unsent_order(
			satellite_id, serial_number,
			order_limit_serialized, order_serialized, order_limit_expiration,
			uplink_cert_id
		) VALUES (?,?, ?,?,?, ?)
	`, info.Limit.SatelliteId, info.Limit.SerialNumber, limitSerialized, orderSerialized, info.Limit.OrderExpiration.UTC(), 0)

	return ErrInfo.Wrap(err)
}

// ListUnsent returns orders that haven't been sent yet.
func (db *ordersdb) ListUnsent(ctx context.Context, limit int) (_ []*orders.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.Query(`
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
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

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		var info orders.Info
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrInfo.Wrap(rows.Err())
}

// ListUnsentBySatellite returns orders that haven't been sent yet grouped by satellite.
// Does not return uplink identity.
func (db *ordersdb) ListUnsentBySatellite(ctx context.Context) (_ map[storj.NodeID][]*orders.Info, err error) {
	defer mon.Task()(&ctx)(&err)
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
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

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
func (db *ordersdb) Archive(ctx context.Context, requests ...orders.ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	txn, err := db.Begin()
	if err != nil {
		return ErrInfo.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = txn.Commit()
		} else {
			err = errs.Combine(err, txn.Rollback())
		}
	}()

	for _, req := range requests {
		err := db.archiveOne(ctx, txn, req)
		if err != nil {
			return ErrInfo.Wrap(err)
		}
	}

	return nil
}

// archiveOne marks order as being handled.
func (db *ordersdb) archiveOne(ctx context.Context, txn *sql.Tx, req orders.ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := txn.Exec(`
		INSERT INTO order_archive_ (
			satellite_id, serial_number,
			order_limit_serialized, order_serialized,
			uplink_cert_id,
			status, archived_at
		) SELECT
			satellite_id, serial_number,
			order_limit_serialized, order_serialized,
			uplink_cert_id,
			?, ?
		FROM unsent_order
		WHERE satellite_id = ? AND serial_number = ?;

		DELETE FROM unsent_order
		WHERE satellite_id = ? AND serial_number = ?;
	`, int(req.Status), time.Now().UTC(), req.Satellite, req.Serial, req.Satellite, req.Serial)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return ErrInfo.Wrap(err)
	}
	if count == 0 {
		return ErrInfo.New("order was not in unsent list")
	}

	return nil
}

// ListArchived returns orders that have been sent.
func (db *ordersdb) ListArchived(ctx context.Context, limit int) (_ []*orders.ArchivedInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.Query(`
		SELECT order_limit_serialized, order_serialized, status, archived_at
		FROM order_archive_
		LIMIT ?
	`, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var infos []*orders.ArchivedInfo
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		var status int
		var archivedAt time.Time

		err := rows.Scan(&limitSerialized, &orderSerialized, &status, &archivedAt)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		var info orders.ArchivedInfo
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		info.Status = orders.Status(status)
		info.ArchivedAt = archivedAt

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrInfo.Wrap(rows.Err())
}
