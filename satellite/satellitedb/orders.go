// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

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

	// sort requests by serial number, all of them should be from the same storage node
	sort.Slice(requests, func(i, k int) bool {
		return requests[i].OrderLimit.SerialNumber.Less(requests[k].OrderLimit.SerialNumber)
	})

	// do a read only transaction to get all the project id/bucket ids
	var bucketIDs [][]byte
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, request := range requests {
			row, err := tx.Find_SerialNumber_By_SerialNumber(ctx,
				dbx.SerialNumber_SerialNumber(request.Order.SerialNumber.Bytes()))
			if err != nil {
				return Error.Wrap(err)
			}
			if row != nil {
				bucketIDs = append(bucketIDs, row.BucketId)
			} else {
				bucketIDs = append(bucketIDs, nil)
			}
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// perform all of the upserts into reported serials table
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		var stmt strings.Builder
		var stmtBegin, stmtEnd string
		switch db.db.implementation {
		case dbutil.Postgres:
			stmtBegin = `INSERT INTO reported_serials ( expires_at, storage_node_id, bucket_id, action, serial_number, settled, observed_at ) VALUES `
			stmtEnd = ` ON CONFLICT ( expires_at, storage_node_id, bucket_id, action, serial_number )
				DO UPDATE SET
					expires_at = EXCLUDED.expires_at,
					storage_node_id = EXCLUDED.storage_node_id,
					bucket_id = EXCLUDED.bucket_id,
					action = EXCLUDED.action,
					serial_number = EXCLUDED.serial_number,
					settled = EXCLUDED.settled,
					observed_at = EXCLUDED.observed_at`
		case dbutil.Cockroach:
			stmtBegin = `UPSERT INTO reported_serials ( expires_at, storage_node_id, bucket_id, action, serial_number, settled, observed_at ) VALUES `
		default:
			return errs.New("invalid dbType: %v", db.db.driver)
		}

		stmt.WriteString(stmtBegin)
		var expiresAt time.Time
		var bucketID []byte
		var serialNum storj.SerialNumber
		var action pb.PieceAction
		var expiresArgNum, bucketArgNum, serialArgNum, actionArgNum int
		var args []interface{}
		args = append(args, storageNodeID.Bytes(), time.Now().UTC())

		for i, request := range requests {
			if bucketIDs[i] == nil {
				responses = append(responses, &orders.ProcessOrderResponse{
					SerialNumber: request.Order.SerialNumber,
					Status:       pb.SettlementResponse_REJECTED,
				})
				continue
			}

			if i > 0 {
				stmt.WriteString(",")
			}
			if expiresAt != roundToNextDay(request.OrderLimit.OrderExpiration) {
				expiresAt = roundToNextDay(request.OrderLimit.OrderExpiration)
				args = append(args, expiresAt)
				expiresArgNum = len(args)
			}
			if string(bucketID) != string(bucketIDs[i]) {
				bucketID = bucketIDs[i]
				args = append(args, bucketID)
				bucketArgNum = len(args)
			}
			if action != request.OrderLimit.Action {
				action = request.OrderLimit.Action
				args = append(args, action)
				actionArgNum = len(args)
			}
			if serialNum != request.Order.SerialNumber {
				serialNum = request.Order.SerialNumber
				args = append(args, serialNum.Bytes())
				serialArgNum = len(args)
			}

			args = append(args, request.Order.Amount)
			stmt.WriteString(fmt.Sprintf(
				"($%d,$1,$%d,$%d,$%d,$%d,$2)",
				expiresArgNum,
				bucketArgNum,
				actionArgNum,
				serialArgNum,
				len(args),
			))

			responses = append(responses, &orders.ProcessOrderResponse{
				SerialNumber: request.Order.SerialNumber,
				Status:       pb.SettlementResponse_ACCEPTED,
			})
		}
		stmt.WriteString(stmtEnd)
		_, err = tx.Tx.ExecContext(ctx, stmt.String(), args...)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	})
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

	const stmtBegin = `
		INSERT INTO bucket_bandwidth_rollups (bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES
	`
	const stmtEnd = `
		ON CONFLICT(bucket_name, project_id, interval_start, action)
		DO UPDATE SET
			allocated = bucket_bandwidth_rollups.allocated + EXCLUDED.allocated,
			inline = bucket_bandwidth_rollups.inline + EXCLUDED.inline,
			settled = bucket_bandwidth_rollups.settled + EXCLUDED.settled
	`

	intervalStart = intervalStart.UTC()
	intervalStart = time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, time.UTC)

	var lastProjectID uuid.UUID
	var lastBucketName string
	var projectIDArgNum int
	var bucketNameArgNum int
	var args []interface{}

	var stmt strings.Builder
	stmt.WriteString(stmtBegin)

	args = append(args, intervalStart)
	for i, rollup := range rollups {
		if i > 0 {
			stmt.WriteString(",")
		}
		if lastProjectID != rollup.ProjectID {
			lastProjectID = rollup.ProjectID
			// Take the slice over a copy of the value so that we don't mutate
			// the underlying value for different range iterations. :grrcox:
			project := rollup.ProjectID
			args = append(args, project[:])
			projectIDArgNum = len(args)
		}
		if lastBucketName != rollup.BucketName {
			lastBucketName = rollup.BucketName
			args = append(args, lastBucketName)
			bucketNameArgNum = len(args)
		}
		args = append(args, rollup.Action, rollup.Inline, rollup.Allocated, rollup.Settled)

		stmt.WriteString(fmt.Sprintf(
			"($%d,$%d,$1,%d,$%d,$%d,$%d,$%d)",
			bucketNameArgNum,
			projectIDArgNum,
			defaultIntervalSeconds,
			len(args)-3,
			len(args)-2,
			len(args)-1,
			len(args),
		))
	}
	stmt.WriteString(stmtEnd)

	_, err = tx.tx.Tx.ExecContext(ctx, stmt.String(), args...)
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

	const stmtBegin = `
		INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, allocated, settled)
		VALUES
	`
	const stmtEnd = `
		ON CONFLICT(storagenode_id, interval_start, action)
		DO UPDATE SET
			allocated = storagenode_bandwidth_rollups.allocated + EXCLUDED.allocated,
			settled = storagenode_bandwidth_rollups.settled + EXCLUDED.settled
	`

	intervalStart = intervalStart.UTC()
	intervalStart = time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), intervalStart.Hour(), 0, 0, 0, time.UTC)

	var lastNodeID storj.NodeID
	var nodeIDArgNum int
	var args []interface{}

	var stmt strings.Builder
	stmt.WriteString(stmtBegin)

	args = append(args, intervalStart)
	for i, rollup := range rollups {
		if i > 0 {
			stmt.WriteString(",")
		}
		if lastNodeID != rollup.NodeID {
			lastNodeID = rollup.NodeID
			// take the slice over rollup.ProjectID, because it is going to stay
			// the same up to the ExecContext call, whereas lastProjectID is likely
			// to be overwritten
			args = append(args, rollup.NodeID.Bytes())
			nodeIDArgNum = len(args)
		}
		args = append(args, rollup.Action, rollup.Allocated, rollup.Settled)

		stmt.WriteString(fmt.Sprintf(
			"($%d,$1,%d,$%d,$%d,$%d)",
			nodeIDArgNum,
			defaultIntervalSeconds,
			len(args)-2,
			len(args)-1,
			len(args),
		))
	}
	stmt.WriteString(stmtEnd)

	_, err = tx.tx.Tx.ExecContext(ctx, stmt.String(), args...)
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
