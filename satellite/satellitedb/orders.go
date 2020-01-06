// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/satellite/orders"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

const defaultIntervalSeconds = int(time.Hour / time.Second)

var (
	// ErrDifferentStorageNodes is returned when ProcessOrders gets orders from different storage nodes.
	ErrDifferentStorageNodes = errs.Class("different storage nodes")
)

type ordersDB struct {
	db *satelliteDB
}

// CreateSerialInfo creates serial number entry in database
func (db *ordersDB) CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.CreateNoReturn_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
		dbx.SerialNumber_BucketId(bucketID),
		dbx.SerialNumber_ExpiresAt(limitExpiration),
	)
}

// DeleteExpiredSerials deletes all expired serials in serial_number and used_serials table.
func (db *ordersDB) DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)
	count, err := db.db.Delete_SerialNumber_By_ExpiresAt_LessOrEqual(ctx, dbx.SerialNumber_ExpiresAt(now))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// UseSerialNumber creates serial number entry in database
func (db *ordersDB) UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO used_serials (serial_number_id, storage_node_id)
		SELECT id, ? FROM serial_numbers WHERE serial_number = ?`,
	)
	_, err = db.db.ExecContext(ctx, statement, storageNodeID.Bytes(), serialNumber.Bytes())
	if err != nil {
		if pgutil.IsConstraintError(err) {
			return nil, orders.ErrUsingSerialNumber.New("serial number already used")
		}
		return nil, err
	}

	dbxSerialNumber, err := db.db.Find_SerialNumber_By_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
	)
	if err != nil {
		return nil, err
	}
	if dbxSerialNumber == nil {
		return nil, orders.ErrUsingSerialNumber.New("serial number not found")
	}
	return dbxSerialNumber.BucketId, nil
}

// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET allocated = bucket_bandwidth_rollups.allocated + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount),
	)
	if err != nil {
		return err
	}

	return nil
}

// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, 0, 0, uint64(amount), uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart, defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpdateStoragenodeBandwidthAllocation updates 'allocated' bandwidth for given storage node
func (db *ordersDB) UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNodes []storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// sort nodes to avoid update deadlock
	sort.Sort(storj.NodeIDList(storageNodes))

	_, err = db.db.ExecContext(ctx, db.db.Rebind(`
		INSERT INTO storagenode_bandwidth_rollups
			(storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		SELECT unnest($1::bytea[]), $2, $3, $4, $5, $6
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET allocated = storagenode_bandwidth_rollups.allocated + excluded.allocated
	`), postgresNodeIDList(storageNodes), intervalStart, defaultIntervalSeconds, action, uint64(amount), 0)

	return Error.Wrap(err)
}

// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node for the given intervalStart time
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, 0, uint64(amount), uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// GetBucketBandwidth gets total bucket bandwidth from period of time
func (db *ordersDB) GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE bucket_name = ? AND project_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(db.db.Rebind(query), bucketName, projectID[:], from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM storagenode_bandwidth_rollups WHERE storagenode_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(db.db.Rebind(query), nodeID.Bytes(), from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// UnuseSerialNumber removes pair serial number -> storage node id from database
func (db *ordersDB) UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := `DELETE FROM used_serials WHERE storage_node_id = ? AND
				  serial_number_id IN (SELECT id FROM serial_numbers WHERE serial_number = ?)`
	_, err = db.db.ExecContext(ctx, db.db.Rebind(statement), storageNodeID.Bytes(), serialNumber.Bytes())
	return err
}

// ProcessOrders take a list of order requests and "settles" them in one transaction.
//
// ProcessOrders requires that all orders come from the same storage node.
func (db *ordersDB) ProcessOrders(ctx context.Context, requests []*orders.ProcessOrderRequest) (responses []*orders.ProcessOrderResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(requests) == 0 {
		return nil, err
	}

	// check that all requests are from the same storage node
	storageNodeID := requests[0].OrderLimit.StorageNodeId
	for _, req := range requests[1:] {
		if req.OrderLimit.StorageNodeId != storageNodeID {
			return nil, ErrDifferentStorageNodes.New("requests from different storage nodes %v and %v", storageNodeID, req.OrderLimit.StorageNodeId)
		}
	}

	// sort requests by serial number, all of them should be from the same storage node
	sort.Slice(requests, func(i, k int) bool {
		return requests[i].OrderLimit.SerialNumber.Less(requests[k].OrderLimit.SerialNumber)
	})

	tx, err := db.db.Begin()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	now := time.Now().UTC()
	intervalStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	rejected := make(map[storj.SerialNumber]bool)
	bucketBySerial := make(map[storj.SerialNumber][]byte)

	// load the bucket id and insert into used serials table
	for _, request := range requests {
		row := tx.QueryRow(db.db.Rebind(`
			SELECT id, bucket_id
			FROM serial_numbers
			WHERE serial_number = ?
		`), request.OrderLimit.SerialNumber)

		var serialNumberID int64
		var bucketID []byte
		if err := row.Scan(&serialNumberID, &bucketID); err != nil {
			rejected[request.OrderLimit.SerialNumber] = true
			continue
		}

		var result sql.Result
		var count int64

		// try to insert the serial number
		result, err = tx.Exec(db.db.Rebind(`
			INSERT INTO used_serials(serial_number_id, storage_node_id)
			VALUES (?, ?)
			ON CONFLICT DO NOTHING
		`), serialNumberID, storageNodeID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		// if we didn't update any rows, then it must already exist
		count, err = result.RowsAffected()
		if err != nil {
			return nil, Error.Wrap(err)
		}
		if count == 0 {
			rejected[request.OrderLimit.SerialNumber] = true
			continue
		}

		bucketBySerial[request.OrderLimit.SerialNumber] = bucketID
	}

	// add up amount by action
	var largestAction pb.PieceAction
	amountByAction := map[pb.PieceAction]int64{}
	for _, request := range requests {
		if rejected[request.OrderLimit.SerialNumber] {
			continue
		}
		limit, order := request.OrderLimit, request.Order
		amountByAction[limit.Action] += order.Amount
		if largestAction < limit.Action {
			largestAction = limit.Action
		}
	}

	// do action updates for storage node
	for action := pb.PieceAction(0); action <= largestAction; action++ {
		amount := amountByAction[action]
		if amount == 0 {
			continue
		}

		_, err := tx.Exec(db.db.Rebind(`
			INSERT INTO storagenode_bandwidth_rollups 
				(storagenode_id, interval_start, interval_seconds, action, allocated, settled)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (storagenode_id, interval_start, action)
			DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?
		`), storageNodeID, intervalStart, defaultIntervalSeconds, action, 0, amount, amount)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	// sort bucket updates
	type bucketUpdate struct {
		bucketID []byte
		action   pb.PieceAction
		amount   int64
	}
	var bucketUpdates []bucketUpdate
	for _, request := range requests {
		if rejected[request.OrderLimit.SerialNumber] {
			continue
		}
		limit, order := request.OrderLimit, request.Order

		bucketUpdates = append(bucketUpdates, bucketUpdate{
			bucketID: bucketBySerial[limit.SerialNumber],
			action:   limit.Action,
			amount:   order.Amount,
		})
	}

	sort.Slice(bucketUpdates, func(i, k int) bool {
		compare := bytes.Compare(bucketUpdates[i].bucketID, bucketUpdates[k].bucketID)
		if compare == 0 {
			return bucketUpdates[i].action < bucketUpdates[k].action
		}
		return compare < 0
	})

	// do bucket updates
	for _, update := range bucketUpdates {
		projectID, bucketName, err := orders.SplitBucketID(update.bucketID)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		_, err = tx.Exec(db.db.Rebind(`
			INSERT INTO bucket_bandwidth_rollups
				(bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (bucket_name, project_id, interval_start, action)
			DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?
		`), bucketName, (*projectID)[:], intervalStart, defaultIntervalSeconds, update.action, 0, 0, update.amount, update.amount)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	for _, request := range requests {
		if !rejected[request.OrderLimit.SerialNumber] {
			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.OrderLimit.SerialNumber,
				Status:       pb.SettlementResponse_ACCEPTED,
			})
		} else {
			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.OrderLimit.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
		}
	}
	return responses, nil
}
