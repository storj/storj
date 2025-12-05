// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/tagsql"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/orders/ordersfile"
)

// ErrOrders represents errors from the ordersdb database.
var ErrOrders = errs.Class("ordersdb")

// OrdersDBName represents the database name.
const OrdersDBName = "orders"

type ordersDB struct {
	dbContainerImpl
}

// Enqueue inserts order to the unsent list.
func (db *ordersDB) Enqueue(ctx context.Context, info *ordersfile.Info) (err error) {
	defer mon.Task()(&ctx)(&err)

	limitSerialized, err := pb.Marshal(info.Limit)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	orderSerialized, err := pb.Marshal(info.Order)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	// TODO: remove uplink_cert_id
	_, err = db.ExecContext(ctx, `
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
func (db *ordersDB) ListUnsent(ctx context.Context, limit int) (_ []*ordersfile.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
		LIMIT ?
	`, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, ErrOrders.Wrap(err)
	}

	var unmarshalErrors errs.Group
	defer func() { err = errs.Combine(err, unmarshalErrors.Err(), rows.Close()) }()

	var infos []*ordersfile.Info
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		var info ordersfile.Info
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		err = pb.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		err = pb.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		infos = append(infos, &info)
	}

	return infos, ErrOrders.Wrap(rows.Err())
}

// ListUnsentBySatellite returns orders that haven't been sent yet and are not expired.
// The orders are ordered by the Satellite ID.
//
// If there is some unmarshal error while reading an order, the method proceed
// with the following ones and the function will return the ones which have
// been successfully read but returning an error with information of the ones
// which have not. In case of database or other system error, the method will
// stop without any further processing and will return an error without any
// order.
func (db *ordersDB) ListUnsentBySatellite(ctx context.Context) (_ map[storj.NodeID][]*ordersfile.Info, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: add some limiting

	rows, err := db.QueryContext(ctx, `
		SELECT order_limit_serialized, order_serialized
		FROM unsent_order
		WHERE order_limit_expiration >= $1
	`, time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, ErrOrders.Wrap(err)
	}

	var unmarshalErrors errs.Group
	defer func() { err = errs.Combine(err, unmarshalErrors.Err(), rows.Close()) }()

	infos := map[storj.NodeID][]*ordersfile.Info{}
	for rows.Next() {
		var limitSerialized []byte
		var orderSerialized []byte

		err := rows.Scan(&limitSerialized, &orderSerialized)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		var info ordersfile.Info
		info.Limit = &pb.OrderLimit{}
		info.Order = &pb.Order{}

		err = pb.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			unmarshalErrors.Add(ErrOrders.Wrap(err))
			continue
		}

		err = pb.Unmarshal(orderSerialized, info.Order)
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

	// change input parameter to UTC timezone before we send it to the database
	archivedAt = archivedAt.UTC()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ErrOrders.Wrap(err)
	}

	var notFoundErrs errs.Group
	defer func() {
		if err == nil {
			err = tx.Commit()
			if err == nil {
				if len(notFoundErrs) > 0 {
					// Return a class error to allow to the caler to identify this case
					err = orders.OrderNotFoundError.Wrap(notFoundErrs.Err())
				}
			}
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	for _, req := range requests {
		err := db.archiveOne(ctx, tx, archivedAt, req)
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
func (db *ordersDB) archiveOne(ctx context.Context, tx tagsql.Tx, archivedAt time.Time, req orders.ArchiveRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := tx.ExecContext(ctx, `
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

	rows, err := db.QueryContext(ctx, `
		SELECT order_limit_serialized, order_serialized, status, archived_at
		FROM order_archive_
		LIMIT ?
	`, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

		err = pb.Unmarshal(limitSerialized, info.Limit)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		err = pb.Unmarshal(orderSerialized, info.Order)
		if err != nil {
			return nil, ErrOrders.Wrap(err)
		}

		infos = append(infos, &info)
	}

	return infos, ErrOrders.Wrap(rows.Err())
}

// CleanArchive deletes all entries older than ttl.
func (db *ordersDB) CleanArchive(ctx context.Context, deleteBefore time.Time) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := db.ExecContext(ctx, `
		DELETE FROM order_archive_
		WHERE archived_at <= ?
	`, deleteBefore.UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
