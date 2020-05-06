// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
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
		return err
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
	if err == sql.ErrNoRows || sum == nil {
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
	if err == sql.ErrNoRows || sum == nil {
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
		`, pq.ByteaArray(serialNums))
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

	// perform all of the upserts into reported serials table
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
				unnest($4::integer[]),
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
				unnest($4::integer[]),
				unnest($5::bigint[]),
				unnest($6::timestamptz[])
		`
	default:
		return nil, Error.New("invalid dbType: %v", db.db.driver)
	}

	_, err = db.db.ExecContext(ctx, stmt,
		storageNodeID.Bytes(),
		pq.ByteaArray(bucketIDArray),
		pq.ByteaArray(serialNumArray),
		pq.Array(actionArray),
		pq.Array(settledArray),
		pq.Array(expiresAtArray),
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
			unnest($5::bigint[]), unnest($6::bigint[]), unnest($7::bigint[]), unnest($8::bigint[])
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
			pq.ByteaArray(projectRUIDs), projectInterval, pq.Array(projectRUAllocated))
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
			unnest($4::bigint[]), unnest($5::bigint[]), unnest($6::bigint[])
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
		pq.ByteaArray(storageNodeIDSlice),
		pq.ByteaArray(serialNumberSlice),
		pq.Array(expiresAtSlice),
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
	db       *satelliteDB
	log      *zap.Logger
	produced []rawPendingSerial
}

func (db *ordersDB) WithQueue(ctx context.Context, cb func(ctx context.Context, queue orders.Queue) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	queue := &ordersDBQueue{
		db:  db.db,
		log: db.db.log,
	}

	err = cb(ctx, queue)
	if err != nil {
		return errs.Wrap(err)
	}

	var nodeIDs, bucketIDs, serialNumbers [][]byte
	for _, pending := range queue.produced {
		nodeIDs = append(nodeIDs, pending.nodeID)
		bucketIDs = append(bucketIDs, pending.bucketID)
		serialNumbers = append(serialNumbers, pending.serialNumber)
	}

	_, err = db.db.ExecContext(ctx, `
			DELETE FROM pending_serial_queue WHERE (
				storage_node_id, bucket_id, serial_number
			) IN (
				SELECT
					unnest($1::bytea[]),
					unnest($2::bytea[]),
					unnest($3::bytea[])
			)
		`,
		pq.ByteaArray(nodeIDs),
		pq.ByteaArray(bucketIDs),
		pq.ByteaArray(serialNumbers))
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (queue *ordersDBQueue) GetPendingSerialsBatch(ctx context.Context, size int) (pendingSerials []orders.PendingSerial, done bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var cont rawPendingSerial
	if len(queue.produced) > 0 {
		cont = queue.produced[len(queue.produced)-1]
	}

	// TODO: this might end up being WORSE on cockroach because it does a hash-join after a
	// full scan of the consumed_serials table, but it's massively better on postgres because
	// it does an indexed anti-join. hopefully we can get rid of the entire serials system
	// before it matters.

	rows, err := queue.db.Query(ctx, `
		SELECT storage_node_id, bucket_id, serial_number, action, settled, expires_at,
			coalesce((
				SELECT 1
				FROM consumed_serials
				WHERE
					consumed_serials.storage_node_id = pending_serial_queue.storage_node_id
					AND consumed_serials.serial_number = pending_serial_queue.serial_number
			), 0) as consumed
		FROM pending_serial_queue
		WHERE (storage_node_id, bucket_id, serial_number) > ($1, $2, $3)
		ORDER BY storage_node_id, bucket_id, serial_number
		LIMIT $4
	`, cont.nodeID, cont.bucketID, cont.serialNumber, size)
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

		queue.produced = append(queue.produced, rawPending)
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
