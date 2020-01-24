// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/dbx"
)

const defaultIntervalSeconds = int(time.Hour / time.Second)

var (
	// ErrDifferentStorageNodes is returned when ProcessOrders gets orders from different storage nodes.
	ErrDifferentStorageNodes = errs.Class("different storage nodes")
)

type ordersDB struct {
	db *satelliteDB

	reportedRollupsReadBatchSize int
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

// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node for the given intervalStart time
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, settled)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		storageNode.Bytes(), intervalStart, defaultIntervalSeconds, action, uint64(amount), uint64(amount),
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
	err = db.db.QueryRow(ctx, db.db.Rebind(query), bucketName, projectID[:], from, to).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, Error.Wrap(err)
}

// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM storagenode_bandwidth_rollups WHERE storagenode_id = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), nodeID.Bytes(), from, to).Scan(&sum)
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
func (db *ordersDB) ProcessOrders(ctx context.Context, requests []*orders.ProcessOrderRequest, observedAt time.Time) (responses []*orders.ProcessOrderResponse, err error) {
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
		`, pq.ByteaArray(serialNums))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Close(), rows.Err()) }()
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

	// perform all of the upserts into reported serials table
	expiresAtArray := make([]time.Time, 0, len(requests))
	bucketIDArray := make([][]byte, 0, len(requests))
	actionArray := make([]pb.PieceAction, 0, len(requests))
	serialNumArray := make([][]byte, 0, len(requests))
	settledArray := make([]int64, 0, len(requests))

	for i, request := range requests {
		if bucketIDs[i] == nil {
			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.Order.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			continue
		}
		expiresAtArray = append(expiresAtArray, roundToNextDay(request.OrderLimit.OrderExpiration))
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
			INSERT INTO reported_serials (
				expires_at, storage_node_id, bucket_id, action, serial_number, settled, observed_at
			)
			SELECT unnest($1::timestamptz[]), $2::bytea, unnest($3::bytea[]), unnest($4::integer[]), unnest($5::bytea[]), unnest($6::bigint[]), $7::timestamptz
			ON CONFLICT ( expires_at, storage_node_id, bucket_id, action, serial_number )
			DO UPDATE SET
				settled = EXCLUDED.settled,
				observed_at = EXCLUDED.observed_at
		`
	case dbutil.Cockroach:
		stmt = `
			UPSERT INTO reported_serials (
				expires_at, storage_node_id, bucket_id, action, serial_number, settled, observed_at
			)
			SELECT unnest($1::timestamptz[]), $2::bytea, unnest($3::bytea[]), unnest($4::integer[]), unnest($5::bytea[]), unnest($6::bigint[]), $7::timestamptz
		`
	default:
		return nil, Error.New("invalid dbType: %v", db.db.driver)
	}

	_, err = db.db.ExecContext(ctx, stmt,
		pq.Array(expiresAtArray),
		storageNodeID.Bytes(),
		pq.ByteaArray(bucketIDArray),
		pq.Array(actionArray),
		pq.ByteaArray(serialNumArray),
		pq.Array(settledArray),
		observedAt.UTC(),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return responses, nil
}

func roundToNextDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
}

// GetBillableBandwidth gets total billable (expired consumed serial) bandwidth for nodes and buckets for all actions.
func (db *ordersDB) GetBillableBandwidth(ctx context.Context, now time.Time) (
	bucketRollups []orders.BucketBandwidthRollup, storagenodeRollups []orders.StoragenodeBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	batchSize := db.reportedRollupsReadBatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	type storagenodeKey struct {
		nodeID storj.NodeID
		action pb.PieceAction
	}
	byStoragenode := make(map[storagenodeKey]uint64)

	type bucketKey struct {
		projectID  uuid.UUID
		bucketName string
		action     pb.PieceAction
	}
	byBucket := make(map[bucketKey]uint64)

	var token *dbx.Paged_ReportedSerial_By_ExpiresAt_LessOrEqual_Continuation
	var rows []*dbx.ReportedSerial

	for {
		// We explicitly use a new transaction each time because we don't need the guarantees and
		// because we don't want a transaction reading for 1000 years.
		rows, token, err = db.db.Paged_ReportedSerial_By_ExpiresAt_LessOrEqual(ctx,
			dbx.ReportedSerial_ExpiresAt(now), batchSize, token)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}

		for _, row := range rows {
			nodeID, err := storj.NodeIDFromBytes(row.StorageNodeId)
			if err != nil {
				db.db.log.Error("bad row inserted into reported serials",
					zap.Binary("storagenode_id", row.StorageNodeId))
				continue
			}
			projectID, bucketName, err := orders.SplitBucketID(row.BucketId)
			if err != nil {
				db.db.log.Error("bad row inserted into reported serials",
					zap.Binary("bucket_id", row.BucketId))
				continue
			}
			action := pb.PieceAction(row.Action)
			settled := row.Settled

			byStoragenode[storagenodeKey{
				nodeID: nodeID,
				action: action,
			}] += settled

			byBucket[bucketKey{
				projectID:  *projectID,
				bucketName: string(bucketName),
				action:     action,
			}] += settled
		}

		if token == nil {
			break
		}
	}

	for key, settled := range byBucket {
		bucketRollups = append(bucketRollups, orders.BucketBandwidthRollup{
			ProjectID:  key.projectID,
			BucketName: key.bucketName,
			Action:     key.action,
			Settled:    int64(settled),
		})
	}

	for key, settled := range byStoragenode {
		storagenodeRollups = append(storagenodeRollups, orders.StoragenodeBandwidthRollup{
			NodeID:  key.nodeID,
			Action:  key.action,
			Settled: int64(settled),
		})
	}

	return bucketRollups, storagenodeRollups, nil
}

//
// transaction/batch methods
//

type ordersDBTx struct {
	tx  *dbx.Tx
	log *zap.Logger
}

func (db *ordersDB) WithTransaction(ctx context.Context, cb func(ctx context.Context, tx orders.Transaction) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		return cb(ctx, &ordersDBTx{tx: tx, log: db.db.log})
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

	for _, rollup := range rollups {
		rollup := rollup
		bucketNames = append(bucketNames, []byte(rollup.BucketName))
		projectIDs = append(projectIDs, rollup.ProjectID[:])
		actionSlice = append(actionSlice, int32(rollup.Action))
		inlineSlice = append(inlineSlice, rollup.Inline)
		allocatedSlice = append(allocatedSlice, rollup.Allocated)
		settledSlice = append(settledSlice, rollup.Settled)
	}

	_, err = tx.tx.Tx.ExecContext(ctx, `
		INSERT INTO bucket_bandwidth_rollups (
			bucket_name, project_id,
			interval_start, interval_seconds,
			action, inline, allocated, settled)
		SELECT
			unnest($1::bytea[]), unnest($2::bytea[]),
			$3, $4,
			unnest($5::integer[]), unnest($6::integer[]), unnest($7::integer[]), unnest($8::integer[])
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET
			allocated = bucket_bandwidth_rollups.allocated + EXCLUDED.allocated,
			inline = bucket_bandwidth_rollups.inline + EXCLUDED.inline,
			settled = bucket_bandwidth_rollups.settled + EXCLUDED.settled`,
		pq.ByteaArray(bucketNames), pq.ByteaArray(projectIDs),
		intervalStart, defaultIntervalSeconds,
		pq.Array(actionSlice), pq.Array(inlineSlice), pq.Array(allocatedSlice), pq.Array(settledSlice))
	if err != nil {
		tx.log.Error("Bucket bandwidth rollup batch flush failed.", zap.Error(err))
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
			unnest($4::integer[]), unnest($5::integer[]), unnest($6::integer[])
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET
			allocated = storagenode_bandwidth_rollups.allocated + EXCLUDED.allocated,
			settled = storagenode_bandwidth_rollups.settled + EXCLUDED.settled`,
		postgresNodeIDList(storageNodeIDs),
		intervalStart, defaultIntervalSeconds,
		pq.Array(actionSlice), pq.Array(allocatedSlice), pq.Array(settledSlice))
	if err != nil {
		tx.log.Error("Storagenode bandwidth rollup batch flush failed.", zap.Error(err))
	}

	return err
}

// DeleteExpiredReportedSerials deletes any expired reported serials as of expiredThreshold.
func (tx *ordersDBTx) DeleteExpiredReportedSerials(ctx context.Context, expiredThreshold time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = tx.tx.Delete_ReportedSerial_By_ExpiresAt_LessOrEqual(ctx,
		dbx.ReportedSerial_ExpiresAt(expiredThreshold))
	return Error.Wrap(err)
}
