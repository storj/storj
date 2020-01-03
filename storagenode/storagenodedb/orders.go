// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/orders"
)

// ErrOrders represents errors from the ordersdb database.
var ErrOrders = errs.Class("ordersdb error")

// OrdersDBName represents the database name.
const OrdersDBName = "orders"

type ordersDB struct {
	dbContainerImpl
}

// Enqueue inserts order to the unsent list
func (db *ordersDB) Enqueue(ctx context.Context, info *orders.Info) (err error) {
	defer mon.Task()(&ctx)(&err)

	limitSerialized, err := proto.Marshal(info.Limit)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	orderSerialized, err := proto.Marshal(info.Order)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	// TODO: remove uplink_cert_id
	_, err = db.Exec(`
		INSERT INTO unsent_order(
			satellite_id, serial_number,
			order_limit_serialized, order_serialized, order_limit_expiration,
			uplink_cert_id
		) VALUES (?,?, ?,?,?, ?)
	`, info.Limit.SatelliteId, info.Limit.SerialNumber, limitSerialized, orderSerialized, info.Limit.OrderExpiration.UTC(), 0)

	return ErrOrders.Wrap(err)
}

// ListUnsent returns orders that haven't been sent yet.
//
// If there is some unmarshal error while reading an order, the method proceed
// with the following ones and the function will return the ones which have
// been successfully read but returning an error with information of the ones
// which have not. In case of database or other system error, the method will
// stop without any further processing and will return an error without any
// order.
func (db *ordersDB) ListUnsent(ctx context.Context, limit int) (_ []*orders.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.Query(`
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
		LIMIT ?
	`, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrOrders.Wrap(err)
	}

	var unmarshalErrors errs.Group
	defer func() { err = errs.Combine(err, unmarshalErrors.Err(), rows.Close()) }()

	var infos []*orders.Info
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		var info orders.Info
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		infos = append(infos, &info)
	}

	return infos, ErrOrders.Wrap(rows.Err())
}

// ListUnsentBySatellite returns orders that haven't been sent yet grouped by
// satellite. Does not return uplink identity.
//
// If there is some unmarshal error while reading an order, the method proceed
// with the following ones and the function will return the ones which have
// been successfully read but returning an error with information of the ones
// which have not. In case of database or other system error, the method will
// stop without any further processing and will return an error without any
// order.
func (db *ordersDB) ListUnsentBySatellite(ctx context.Context) (_ map[storj.NodeID][]*orders.Info, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: add some limiting

	rows, err := db.Query(`
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
	`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrOrders.Wrap(err)
	}

	var unmarshalErrors errs.Group
	defer func() { err = errs.Combine(err, unmarshalErrors.Err(), rows.Close()) }()

	infos := map[storj.NodeID][]*orders.Info{}
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		var info orders.Info
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		infos[info.Limit.SatelliteId] = append(infos[info.Limit.SatelliteId], &info)
	}

	return infos, ErrOrders.Wrap(rows.Err())
}

// Archive marks order as being handled.
//
// If any of the request contains an order which doesn't exist the method will
// follow with the next ones without interrupting the operation and it will
// return an error of the class orders.OrderNotFoundError. Any other error, will
// abort the operation, rolling back the transaction.
func (db *ordersDB) Archive(ctx context.Context, archivedAt time.Time, requests ...orders.ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	txn, err := db.Begin()
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	var notFoundErrs errs.Group
	defer func() {
		if err == nil {
			err = txn.Commit()
			if err == nil {
				if len(notFoundErrs) > 0 {
					// Return a class error to allow to the caler to identify this case
					err = orders.OrderNotFoundError.Wrap(notFoundErrs.Err())
				}
			}
		} else {
			err = errs.Combine(err, txn.Rollback())
		}
	}()

	for _, req := range requests {
		err := db.archiveOne(ctx, txn, archivedAt, req)
		if err != nil {
			if orders.OrderNotFoundError.Has(err) {
				notFoundErrs.Add(err)
				continue
			}

			return err
		}
	}

	return nil
}

// archiveOne marks order as being handled.
func (db *ordersDB) archiveOne(ctx context.Context, txn *sql.Tx, archivedAt time.Time, req orders.ArchiveRequest) (err error) {
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
	`, int(req.Status), archivedAt, req.Satellite, req.Serial, req.Satellite, req.Serial)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return ErrOrders.Wrap(err)
	}
	if count == 0 {
		return orders.OrderNotFoundError.New("satellite: %s, serial number: %s",
			req.Satellite.String(), req.Serial.String(),
		)
	}

	return nil
}

// ListArchived returns orders that have been sent.
func (db *ordersDB) ListArchived(ctx context.Context, limit int) (_ []*orders.ArchivedInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.Query(`
		SELECT order_limit_serialized, order_serialized, status, archived_at
		FROM order_archive_
		LIMIT ?
	`, limit)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrOrders.Wrap(err)
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
			return nil, ErrOrders.Wrap(err)
		}

		var info orders.ArchivedInfo
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		info.Status = orders.Status(status)
		info.ArchivedAt = archivedAt

		err = proto.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		err = proto.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrOrders.Wrap(rows.Err())
}

// CleanArchive deletes all entries older than ttl
func (db *ordersDB) CleanArchive(ctx context.Context, ttl time.Duration) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)

	deleteBefore := time.Now().UTC().Add(-1 * ttl)
	result, err := db.Exec(`
		DELETE FROM order_archive_
		WHERE archived_at <= ?
	`, deleteBefore)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, ErrOrders.Wrap(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, ErrOrders.Wrap(err)
	}
	return int(count), nil
}
