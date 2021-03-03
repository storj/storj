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
	"storj.io/storj/private/dbutil/pgutil"
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

	var sum1, sum2 int64

	err1 := db.db.QueryRow(ctx, db.db.Rebind(`
		SELECT COALESCE(SUM(settled), 0)
		FROM storagenode_bandwidth_rollups
		WHERE storagenode_id = ?
		  AND interval_start > ?
		  AND interval_start <= ?
	`), nodeID.Bytes(), from.UTC(), to.UTC()).Scan(&sum1)

	err2 := db.db.QueryRow(ctx, db.db.Rebind(`
		SELECT COALESCE(SUM(settled), 0)
		FROM storagenode_bandwidth_rollups_phase2
		WHERE storagenode_id = ?
		  AND interval_start > ?
		  AND interval_start <= ?
	`), nodeID.Bytes(), from.UTC(), to.UTC()).Scan(&sum2)

	if err1 != nil && !errors.Is(err1, sql.ErrNoRows) {
		return 0, err1
	} else if err2 != nil && !errors.Is(err2, sql.ErrNoRows) {
		return 0, err2
	}

	return sum1 + sum2, nil
}

func (db *ordersDB) UpdateBucketBandwidthBatch(ctx context.Context, intervalStart time.Time, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
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
		projectRUMap := make(map[string]int64)

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

		_, err = tx.Tx.ExecContext(ctx, `
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
			db.db.log.Error("Bucket bandwidth rollup batch flush failed.", zap.Error(err))
		}

		var projectRUIDs [][]byte
		var projectRUAllocated []int64
		projectInterval := time.Date(intervalStart.Year(), intervalStart.Month(), 1, intervalStart.Hour(), 0, 0, 0, time.UTC)

		for k, v := range projectRUMap {
			projectID, err := uuid.FromString(k)
			if err != nil {
				db.db.log.Error("Could not parse project UUID.", zap.Error(err))
				continue
			}
			projectRUIDs = append(projectRUIDs, projectID[:])
			projectRUAllocated = append(projectRUAllocated, v)
		}

		if len(projectRUIDs) > 0 {
			_, err = tx.Tx.ExecContext(ctx, `
		INSERT INTO project_bandwidth_rollups(project_id, interval_month, egress_allocated)
			SELECT unnest($1::bytea[]), $2, unnest($3::bigint[])
		ON CONFLICT(project_id, interval_month)
		DO UPDATE SET egress_allocated = project_bandwidth_rollups.egress_allocated + EXCLUDED.egress_allocated::bigint;
		`,
				pgutil.ByteaArray(projectRUIDs), projectInterval, pgutil.Int8Array(projectRUAllocated))
			if err != nil {
				db.db.log.Error("Project bandwidth rollup batch flush failed.", zap.Error(err))
			}
		}
		return err
	})
}

//
// transaction/batch methods
//

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
	rowsSumByAction := map[int32]int64{}
	for _, row := range rows {
		rowsSumByAction[int32(row.Action)] += int64(row.Settled)
	}

	return reflect.DeepEqual(rowsSumByAction, orderActionAmounts)
}
