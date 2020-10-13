// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/dbx"
)

const defaultIntervalSeconds = int(time.Hour / time.Second)

var (
	// ErrDifferentStorageNodes is returned when ProcessOrders gets orders from different storage nodes.
	ErrDifferentStorageNodes = errs.Class("different storage nodes")
	// ErrBucketFromSerial is returned when there is an error trying to get the bucket name from the serial number.
	ErrBucketFromSerial = errs.Class("bucket from serial number")
	// ErrUpdateBucketBandwidthSettle is returned when there is an error updating bucket bandwidth.
	ErrUpdateBucketBandwidthSettle = errs.Class("update bucket bandwidth settle")
	// ErrProcessOrderWithWindowTx is returned when there is an error with the ProcessOrders transaction.
	ErrProcessOrderWithWindowTx = errs.Class("process order with window transaction")
	// ErrGetStoragenodeBandwidthInWindow is returned when there is an error getting all storage node bandwidth for a window.
	ErrGetStoragenodeBandwidthInWindow = errs.Class("get storagenode bandwidth in window")
	// ErrCreateStoragenodeBandwidth is returned when there is an error updating storage node bandwidth.
	ErrCreateStoragenodeBandwidth = errs.Class("create storagenode bandwidth")
)

type ordersDB struct {
	db *satelliteDB

	reportedRollupsReadBatchSize int
}

// CreateSerialInfo creates serial number entry in database.
func (db *ordersDB) CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.CreateNoReturn_SerialNumber(
		ctx,
		dbx.SerialNumber_SerialNumber(serialNumber.Bytes()),
		dbx.SerialNumber_BucketId(bucketID),
		dbx.SerialNumber_ExpiresAt(limitExpiration.UTC()),
	)
}

// DeleteExpiredSerials deletes all expired serials in serial_number and used_serials table.
func (db *ordersDB) DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)

	count, err := db.db.Delete_SerialNumber_By_ExpiresAt_LessOrEqual(ctx, dbx.SerialNumber_ExpiresAt(now.UTC()))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// DeleteExpiredConsumedSerials deletes all expired serials in the consumed_serials table.
func (db *ordersDB) DeleteExpiredConsumedSerials(ctx context.Context, now time.Time) (_ int, err error) {
	defer mon.Task()(&ctx, now)(&err)

	count, err := db.db.Delete_ConsumedSerial_By_ExpiresAt_LessOrEqual(ctx, dbx.ConsumedSerial_ExpiresAt(now))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// UseSerialNumber creates a used serial number entry in database from an
// existing serial number.
// It returns the bucket ID associated to serialNumber.
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

// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		statement := tx.Rebind(
			`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(bucket_name, project_id, interval_start, action)
			DO UPDATE SET allocated = bucket_bandwidth_rollups.allocated + ?`,
		)
		_, err = tx.Tx.ExecContext(ctx, statement,
			bucketName, projectID[:], intervalStart.UTC(), defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount),
		)
		if err != nil {
			return err
		}

		if action == pb.PieceAction_GET {
			projectInterval := time.Date(intervalStart.Year(), intervalStart.Month(), 1, 0, 0, 0, 0, time.UTC)
			statement = tx.Rebind(
				`INSERT INTO project_bandwidth_rollups (project_id, interval_month, egress_allocated)
				VALUES (?, ?, ?)
				ON CONFLICT(project_id, interval_month)
				DO UPDATE SET egress_allocated = project_bandwidth_rollups.egress_allocated + EXCLUDED.egress_allocated::bigint`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement, projectID[:], projectInterval, uint64(amount))
			if err != nil {
				return err
			}
		}
		return nil
	})

	return nil
}

// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart.UTC(), defaultIntervalSeconds, action, 0, 0, uint64(amount), uint64(amount),
	)
	if err != nil {
		return ErrUpdateBucketBandwidthSettle.Wrap(err)
	}
	return nil
}

// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		bucketName, projectID[:], intervalStart.UTC(), defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node for the given intervalStart time.
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, settled)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		storageNode.Bytes(), intervalStart.UTC(), defaultIntervalSeconds, action, uint64(amount), uint64(amount),
	)
	if err != nil {
		return err
	}
	return nil
}

// GetBucketBandwidth gets total bucket bandwidth from period of time.
func (db *ordersDB) GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE bucket_name = ? AND project_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), bucketName, projectID[:], from.UTC(), to.UTC()).Scan(&sum)
	if errors.Is(err, sql.ErrNoRows) || sum == nil {
		return 0, nil
	}
	return *sum, Error.Wrap(err)
}

// GetStorageNodeBandwidth gets total storage node bandwidth from period of time.
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var sum *int64
	query := `SELECT SUM(settled) FROM storagenode_bandwidth_rollups WHERE storagenode_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), nodeID.Bytes(), from.UTC(), to.UTC()).Scan(&sum)
	if errors.Is(err, sql.ErrNoRows) || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// UnuseSerialNumber removes pair serial number -> storage node id from database.
func (db *ordersDB) UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := `DELETE FROM used_serials WHERE storage_node_id = ? AND
				  serial_number_id IN (SELECT id FROM serial_numbers WHERE serial_number = ?)`
	_, err = db.db.ExecContext(ctx, db.db.Rebind(statement), storageNodeID.Bytes(), serialNumber.Bytes())
	return err
}

// ProcessOrders take a list of order requests and inserts them into the pending serials queue.
//
// ProcessOrders requires that all orders come from the same storage node.
func (db *ordersDB) ProcessOrders(ctx context.Context, requests []*orders.ProcessOrderRequest) (responses []*orders.ProcessOrderResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(requests) == 0 {
		return nil, nil
	}

	// check that all requests are from the same storage node
	storageNodeID := requests[0].OrderLimit.StorageNodeId
	for _, req := range requests[1:] {
		if req.OrderLimit.StorageNodeId != storageNodeID {
			return nil, ErrDifferentStorageNodes.New("requests from different storage nodes %v and %v", storageNodeID, req.OrderLimit.StorageNodeId)
		}
	}

	// Do a read first to get all the project id/bucket ids. We could combine this with the
	// upsert below by doing a join, but there isn't really any need for special consistency
	// semantics between these two queries, and it should make things easier on the database
	// (particularly cockroachDB) to have the freedom to perform them separately.
	//
	// We don't expect the serial_number -> bucket_id relationship ever to change, as long as a
	// serial_number exists. There is a possibility of a serial_number being deleted between
	// this query and the next, but that is ok too (rows in reported_serials may end up having
	// serial numbers that no longer exist in serial_numbers, but that shouldn't break
	// anything.)
	bucketIDs, err := func() (bucketIDs [][]byte, err error) {
		bucketIDs = make([][]byte, len(requests))
		serialNums := make([][]byte, len(requests))
		for i, request := range requests {
			serialNums[i] = request.Order.SerialNumber.Bytes()
		}
		rows, err := db.db.QueryContext(ctx, `
			SELECT request.i, sn.bucket_id
			FROM
				serial_numbers sn,
				unnest($1::bytea[]) WITH ORDINALITY AS request(serial_number, i)
			WHERE request.serial_number = sn.serial_number
		`, pgutil.ByteaArray(serialNums))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()
		for rows.Next() {
			var index int
			var bucketID []byte
			err = rows.Scan(&index, &bucketID)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			bucketIDs[index-1] = bucketID
		}
		return bucketIDs, nil
	}()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// perform all of the upserts into pending_serial_queue table
	expiresAtArray := make([]time.Time, 0, len(requests))
	bucketIDArray := make([][]byte, 0, len(requests))
	actionArray := make([]pb.PieceAction, 0, len(requests))
	serialNumArray := make([][]byte, 0, len(requests))
	settledArray := make([]int64, 0, len(requests))

	// remove duplicate bucket_id, serial_number pairs sent in the same request.
	// postgres will complain.
	type requestKey struct {
		BucketID     string
		SerialNumber storj.SerialNumber
	}
	seenRequests := make(map[requestKey]struct{})

	for i, request := range requests {
		if bucketIDs[i] == nil {
			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.Order.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			continue
		}

		// Filter duplicate requests and reject them.
		key := requestKey{
			BucketID:     string(bucketIDs[i]),
			SerialNumber: request.Order.SerialNumber,
		}
		if _, seen := seenRequests[key]; seen {
			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.Order.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			continue
		}
		seenRequests[key] = struct{}{}

		expiresAtArray = append(expiresAtArray, request.OrderLimit.OrderExpiration)
		bucketIDArray = append(bucketIDArray, bucketIDs[i])
		actionArray = append(actionArray, request.OrderLimit.Action)
		serialNumCopy := request.Order.SerialNumber
		serialNumArray = append(serialNumArray, serialNumCopy[:])
		settledArray = append(settledArray, request.Order.Amount)

		responses = append(responses, &orders.ProcessOrderResponse{
			SerialNumber: request.Order.SerialNumber,
			Status:       pb.SettlementResponse_ACCEPTED,
		})
	}

	var stmt string
	switch db.db.implementation {
	case dbutil.Postgres:
		stmt = `
			INSERT INTO pending_serial_queue (
				storage_node_id, bucket_id, serial_number, action, settled, expires_at
			)
			SELECT
				$1::bytea,
				unnest($2::bytea[]),
				unnest($3::bytea[]),
				unnest($4::int4[]),
				unnest($5::bigint[]),
				unnest($6::timestamptz[])
			ON CONFLICT ( storage_node_id, bucket_id, serial_number )
			DO UPDATE SET
				action = EXCLUDED.action,
				settled = EXCLUDED.settled,
				expires_at = EXCLUDED.expires_at
		`
	case dbutil.Cockroach:
		stmt = `
			UPSERT INTO pending_serial_queue (
				storage_node_id, bucket_id, serial_number, action, settled, expires_at
			)
			SELECT
				$1::bytea,
				unnest($2::bytea[]),
				unnest($3::bytea[]),
				unnest($4::int4[]),
				unnest($5::bigint[]),
				unnest($6::timestamptz[])
		`
	default:
		return nil, Error.New("invalid dbType: %v", db.db.driver)
	}

	actionNumArray := make([]int32, len(actionArray))
	for i, num := range actionArray {
		actionNumArray[i] = int32(num)
	}

	_, err = db.db.ExecContext(ctx, stmt,
		storageNodeID.Bytes(),
		pgutil.ByteaArray(bucketIDArray),
		pgutil.ByteaArray(serialNumArray),
		pgutil.Int4Array(actionNumArray),
		pgutil.Int8Array(settledArray),
		pgutil.TimestampTZArray(expiresAtArray),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return responses, nil
}

//
// transaction/batch methods
//

type ordersDBTx struct {
	tx  *dbx.Tx
	db  *satelliteDB
	log *zap.Logger
}

func (db *ordersDB) WithTransaction(ctx context.Context, cb func(ctx context.Context, tx orders.Transaction) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		return cb(ctx, &ordersDBTx{tx: tx, db: db.db, log: db.db.log})
	})
}

func (tx *ordersDBTx) UpdateBucketBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(rollups) == 0 {
		return nil
	}

	orders.SortBucketBandwidthRollups(rollups)

	intervalStart = intervalStart.UTC()
	intervalStart = time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, time.UTC)

	var bucketNames [][]byte
	var projectIDs [][]byte
	var actionSlice []int32
	var inlineSlice []int64
	var allocatedSlice []int64
	var settledSlice []int64
	var projectRUMap map[string]int64 = make(map[string]int64)

	for _, rollup := range rollups {
		rollup := rollup
		bucketNames = append(bucketNames, []byte(rollup.BucketName))
		projectIDs = append(projectIDs, rollup.ProjectID[:])
		actionSlice = append(actionSlice, int32(rollup.Action))
		inlineSlice = append(inlineSlice, rollup.Inline)
		allocatedSlice = append(allocatedSlice, rollup.Allocated)
		settledSlice = append(settledSlice, rollup.Settled)

		if rollup.Action == pb.PieceAction_GET {
			projectRUMap[rollup.ProjectID.String()] += rollup.Allocated
		}
	}

	_, err = tx.tx.Tx.ExecContext(ctx, `
		INSERT INTO bucket_bandwidth_rollups (
			bucket_name, project_id,
			interval_start, interval_seconds,
			action, inline, allocated, settled)
		SELECT
			unnest($1::bytea[]), unnest($2::bytea[]),
			$3, $4,
			unnest($5::int4[]), unnest($6::bigint[]), unnest($7::bigint[]), unnest($8::bigint[])
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET
			allocated = bucket_bandwidth_rollups.allocated + EXCLUDED.allocated,
			inline = bucket_bandwidth_rollups.inline + EXCLUDED.inline,
			settled = bucket_bandwidth_rollups.settled + EXCLUDED.settled`,
		pgutil.ByteaArray(bucketNames), pgutil.ByteaArray(projectIDs),
		intervalStart, defaultIntervalSeconds,
		pgutil.Int4Array(actionSlice), pgutil.Int8Array(inlineSlice), pgutil.Int8Array(allocatedSlice), pgutil.Int8Array(settledSlice))
	if err != nil {
		tx.log.Error("Bucket bandwidth rollup batch flush failed.", zap.Error(err))
	}

	var projectRUIDs [][]byte
	var projectRUAllocated []int64
	projectInterval := time.Date(intervalStart.Year(), intervalStart.Month(), 1, intervalStart.Hour(), 0, 0, 0, time.UTC)

	for k, v := range projectRUMap {
		projectID, err := uuid.FromString(k)
		if err != nil {
			tx.log.Error("Could not parse project UUID.", zap.Error(err))
			continue
		}
		projectRUIDs = append(projectRUIDs, projectID[:])
		projectRUAllocated = append(projectRUAllocated, v)
	}

	if len(projectRUIDs) > 0 {
		_, err = tx.tx.Tx.ExecContext(ctx, `
		INSERT INTO project_bandwidth_rollups(project_id, interval_month, egress_allocated)
			SELECT unnest($1::bytea[]), $2, unnest($3::bigint[])
		ON CONFLICT(project_id, interval_month)
		DO UPDATE SET egress_allocated = project_bandwidth_rollups.egress_allocated + EXCLUDED.egress_allocated::bigint;
		`,
			pgutil.ByteaArray(projectRUIDs), projectInterval, pgutil.Int8Array(projectRUAllocated))
		if err != nil {
			tx.log.Error("Project bandwidth rollup batch flush failed.", zap.Error(err))
		}
	}
	return err
}

func (tx *ordersDBTx) UpdateStoragenodeBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []orders.StoragenodeBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(rollups) == 0 {
		return nil
	}

	orders.SortStoragenodeBandwidthRollups(rollups)

	var storageNodeIDs []storj.NodeID
	var actionSlice []int32
	var allocatedSlice []int64
	var settledSlice []int64

	intervalStart = intervalStart.UTC()
	intervalStart = time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, time.UTC)

	for i := range rollups {
		rollup := &rollups[i]
		storageNodeIDs = append(storageNodeIDs, rollup.NodeID)
		actionSlice = append(actionSlice, int32(rollup.Action))
		allocatedSlice = append(allocatedSlice, rollup.Allocated)
		settledSlice = append(settledSlice, rollup.Settled)
	}

	_, err = tx.tx.Tx.ExecContext(ctx, `
		INSERT INTO storagenode_bandwidth_rollups(
			storagenode_id,
			interval_start, interval_seconds,
			action, allocated, settled)
		SELECT
			unnest($1::bytea[]),
			$2, $3,
			unnest($4::int4[]), unnest($5::bigint[]), unnest($6::bigint[])
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET
			allocated = storagenode_bandwidth_rollups.allocated + EXCLUDED.allocated,
			settled = storagenode_bandwidth_rollups.settled + EXCLUDED.settled`,
		pgutil.NodeIDArray(storageNodeIDs),
		intervalStart, defaultIntervalSeconds,
		pgutil.Int4Array(actionSlice), pgutil.Int8Array(allocatedSlice), pgutil.Int8Array(settledSlice))
	if err != nil {
		tx.log.Error("Storagenode bandwidth rollup batch flush failed.", zap.Error(err))
	}

	return err
}

// CreateConsumedSerialsBatch creates a batch of consumed serial entries.
func (tx *ordersDBTx) CreateConsumedSerialsBatch(ctx context.Context, consumedSerials []orders.ConsumedSerial) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(consumedSerials) == 0 {
		return nil
	}

	var storageNodeIDSlice [][]byte
	var serialNumberSlice [][]byte
	var expiresAtSlice []time.Time

	for _, consumedSerial := range consumedSerials {
		consumedSerial := consumedSerial
		storageNodeIDSlice = append(storageNodeIDSlice, consumedSerial.NodeID.Bytes())
		serialNumberSlice = append(serialNumberSlice, consumedSerial.SerialNumber.Bytes())
		expiresAtSlice = append(expiresAtSlice, consumedSerial.ExpiresAt)
	}

	var stmt string
	switch tx.db.implementation {
	case dbutil.Postgres:
		stmt = `
			INSERT INTO consumed_serials (
				storage_node_id, serial_number, expires_at
			)
			SELECT unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::timestamptz[])
			ON CONFLICT ( storage_node_id, serial_number ) DO NOTHING
		`
	case dbutil.Cockroach:
		stmt = `
			UPSERT INTO consumed_serials (
				storage_node_id, serial_number, expires_at
			)
			SELECT unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::timestamptz[])
		`
	default:
		return Error.New("invalid dbType: %v", tx.db.driver)
	}

	_, err = tx.tx.Tx.ExecContext(ctx, stmt,
		pgutil.ByteaArray(storageNodeIDSlice),
		pgutil.ByteaArray(serialNumberSlice),
		pgutil.TimestampTZArray(expiresAtSlice),
	)
	return Error.Wrap(err)
}

func (tx *ordersDBTx) HasConsumedSerial(ctx context.Context, nodeID storj.NodeID, serialNumber storj.SerialNumber) (exists bool, err error) {
	defer mon.Task()(&ctx)(&err)

	exists, err = tx.tx.Has_ConsumedSerial_By_StorageNodeId_And_SerialNumber(ctx,
		dbx.ConsumedSerial_StorageNodeId(nodeID.Bytes()),
		dbx.ConsumedSerial_SerialNumber(serialNumber.Bytes()))
	return exists, Error.Wrap(err)
}

//
// transaction/batch methods
//

type rawPendingSerial struct {
	nodeID       []byte
	bucketID     []byte
	serialNumber []byte
}

type ordersDBQueue struct {
	impl dbutil.Implementation
	log  *zap.Logger
	tx   tagsql.Tx
}

func (db *ordersDB) WithQueue(ctx context.Context, cb func(ctx context.Context, queue orders.Queue) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		return cb(ctx, &ordersDBQueue{
			impl: db.db.implementation,
			log:  db.db.log,
			tx:   tx.Tx,
		})
	}))
}

func (queue *ordersDBQueue) GetPendingSerialsBatch(ctx context.Context, size int) (pendingSerials []orders.PendingSerial, done bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: no idea of this query makes sense on cockroach. it may do a terrible job with it.
	// but it's blazing fast on postgres and that's where we have the problem! :D :D :D

	var rows tagsql.Rows
	switch queue.impl {
	case dbutil.Postgres:
		rows, err = queue.tx.Query(ctx, `
			DELETE
				FROM pending_serial_queue
				WHERE
					ctid = any (array(
						SELECT
							ctid
						FROM pending_serial_queue
						LIMIT $1
					))
				RETURNING storage_node_id, bucket_id, serial_number, action, settled, expires_at, (
					coalesce((
						SELECT 1
						FROM consumed_serials
						WHERE
							consumed_serials.storage_node_id = pending_serial_queue.storage_node_id
							AND consumed_serials.serial_number = pending_serial_queue.serial_number
					), 0))
		`, size)
	case dbutil.Cockroach:
		rows, err = queue.tx.Query(ctx, `
			DELETE
				FROM pending_serial_queue
				WHERE
					(storage_node_id, bucket_id, serial_number) = any (array(
						SELECT
							(storage_node_id, bucket_id, serial_number)
						FROM pending_serial_queue
						LIMIT $1
					))
				RETURNING storage_node_id, bucket_id, serial_number, action, settled, expires_at, (
					coalesce((
						SELECT 1
						FROM consumed_serials
						WHERE
							consumed_serials.storage_node_id = pending_serial_queue.storage_node_id
							AND consumed_serials.serial_number = pending_serial_queue.serial_number
					), 0))
		`, size)
	default:
		return nil, false, Error.New("unhandled implementation")
	}
	if err != nil {
		return nil, false, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, Error.Wrap(rows.Close())) }()

	for rows.Next() {
		var consumed int
		var rawPending rawPendingSerial
		var pendingSerial orders.PendingSerial

		err := rows.Scan(
			&rawPending.nodeID,
			&rawPending.bucketID,
			&rawPending.serialNumber,
			&pendingSerial.Action,
			&pendingSerial.Settled,
			&pendingSerial.ExpiresAt,
			&consumed,
		)
		if err != nil {
			return nil, false, Error.Wrap(err)
		}

		size--

		if consumed != 0 {
			continue
		}

		pendingSerial.NodeID, err = storj.NodeIDFromBytes(rawPending.nodeID)
		if err != nil {
			queue.log.Error("Invalid storage node id in pending serials queue",
				zap.Binary("id", rawPending.nodeID),
				zap.Error(errs.Wrap(err)))
			continue
		}
		pendingSerial.BucketID = rawPending.bucketID
		pendingSerial.SerialNumber, err = storj.SerialNumberFromBytes(rawPending.serialNumber)
		if err != nil {
			queue.log.Error("Invalid serial number in pending serials queue",
				zap.Binary("id", rawPending.serialNumber),
				zap.Error(errs.Wrap(err)))
			continue
		}

		pendingSerials = append(pendingSerials, pendingSerial)
	}
	if err := rows.Err(); err != nil {
		return nil, false, Error.Wrap(err)
	}

	return pendingSerials, size > 0, nil
}

// UpdateStoragenodeBandwidthSettleWithWindow adds a record to for each action and settled amount.
// If any of these orders already exist in the database, then all of these orders have already been processed.
// Orders within a single window may only be processed once to prevent double spending.
func (db *ordersDB) UpdateStoragenodeBandwidthSettleWithWindow(ctx context.Context, storageNodeID storj.NodeID, actionAmounts map[int32]int64, window time.Time) (status pb.SettlementWithWindowResponse_Status, alreadyProcessed bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var batchStatus pb.SettlementWithWindowResponse_Status
	var retryCount int
	for {
		err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			// try to get all rows from the storage node bandwidth table for the 1 hr window
			// if there are already existing rows for the 1 hr window that means these orders have
			// already been processed
			rows, err := tx.All_StoragenodeBandwidthRollup_By_StoragenodeId_And_IntervalStart(ctx,
				dbx.StoragenodeBandwidthRollup_StoragenodeId(storageNodeID[:]),
				dbx.StoragenodeBandwidthRollup_IntervalStart(window),
			)
			if err != nil {
				return ErrGetStoragenodeBandwidthInWindow.Wrap(err)
			}

			if len(rows) != 0 {
				// if there are already rows in the storagenode bandwidth table for this 1 hr window
				// that means these orders have already been processed
				// if these orders that the storagenode is trying to process again match what in the
				// storagenode bandwidth table, then send a successful response to the storagenode
				// so they don't keep trying to settle these orders again
				// if these orders do not match what we have in the storage node bandwidth table then send
				// back an invalid response
				if SettledAmountsMatch(rows, actionAmounts) {
					batchStatus = pb.SettlementWithWindowResponse_ACCEPTED
					alreadyProcessed = true
					return nil
				}
				batchStatus = pb.SettlementWithWindowResponse_REJECTED
				return nil
			}
			// if there aren't any rows in the storagenode bandwidth table for this 1 hr window
			// that means these orders have not been processed before so we can continue to process them
			for action, amount := range actionAmounts {
				_, err := tx.Create_StoragenodeBandwidthRollup(ctx,
					dbx.StoragenodeBandwidthRollup_StoragenodeId(storageNodeID[:]),
					dbx.StoragenodeBandwidthRollup_IntervalStart(window),
					dbx.StoragenodeBandwidthRollup_IntervalSeconds(uint(defaultIntervalSeconds)),
					dbx.StoragenodeBandwidthRollup_Action(uint(action)),
					dbx.StoragenodeBandwidthRollup_Settled(uint64(amount)),
					dbx.StoragenodeBandwidthRollup_Create_Fields{},
				)
				if err != nil {
					return ErrCreateStoragenodeBandwidth.Wrap(err)
				}
			}

			batchStatus = pb.SettlementWithWindowResponse_ACCEPTED
			return nil
		})
		if errs.IsFunc(err, dbx.IsConstraintError) {
			retryCount++
			if retryCount > 5 {
				return 0, alreadyProcessed, errs.New("process order with window retry count too high")
			}
			continue
		} else if err != nil {
			return 0, alreadyProcessed, ErrProcessOrderWithWindowTx.Wrap(err)
		}
		break
	}

	return batchStatus, alreadyProcessed, nil
}

// SettledAmountsMatch checks if database rows match the orders. If the settled amount for
// each action are not the same then false is returned.
func SettledAmountsMatch(rows []*dbx.StoragenodeBandwidthRollup, orderActionAmounts map[int32]int64) bool {
	var rowsSumByAction = map[int32]int64{}
	for _, row := range rows {
		rowsSumByAction[int32(row.Action)] += int64(row.Settled)
	}

	return reflect.DeepEqual(rowsSumByAction, orderActionAmounts)
}

func (db *ordersDB) GetBucketIDFromSerialNumber(ctx context.Context, serialNumber storj.SerialNumber) ([]byte, error) {
	row, err := db.db.Get_SerialNumber_BucketId_By_SerialNumber(ctx,
		dbx.SerialNumber_SerialNumber(serialNumber[:]),
	)
	if err != nil {
		return nil, ErrBucketFromSerial.Wrap(err)
	}
	return row.BucketId, nil
}
