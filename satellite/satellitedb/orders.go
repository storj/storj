// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgxutil"
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

type bandwidth struct {
	Allocated int64
	Settled   int64
	Inline    int64
	Dead      int64
}

type bandwidthRollupKey struct {
	BucketName    string
	ProjectID     uuid.UUID
	IntervalStart int64
	Action        pb.PieceAction
}

// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO I wanted to remove this implementation but it looks it's heavily used in tests
	// we should do cleanup as a separate change (Michal)

	return pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch

		// TODO decide if we need to have transaction here
		batch.Queue(`START TRANSACTION`)

		statement := db.db.Rebind(
			`INSERT INTO bucket_bandwidth_rollups (project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(project_id, bucket_name, interval_start, action)
					DO UPDATE SET allocated = bucket_bandwidth_rollups.allocated + ?`,
		)
		batch.Queue(statement, projectID[:], bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount))

		if action == pb.PieceAction_GET {
			dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)
			statement = db.db.Rebind(
				`INSERT INTO project_bandwidth_daily_rollups (project_id, interval_day, egress_allocated, egress_settled, egress_dead)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(project_id, interval_day)
				DO UPDATE SET egress_allocated = project_bandwidth_daily_rollups.egress_allocated + EXCLUDED.egress_allocated::BIGINT`,
			)
			batch.Queue(statement, projectID[:], dailyInterval, uint64(amount), 0, 0)
		}

		batch.Queue(`COMMIT TRANSACTION`)

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		var errlist errs.Group
		for i := 0; i < batch.Len(); i++ {
			_, err := results.Exec()
			errlist.Add(err)
		}

		return errlist.Err()
	})
}

// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, settledAmount, deadAmount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		statement := tx.Rebind(
			`INSERT INTO bucket_bandwidth_rollups (project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(project_id, bucket_name, interval_start, action)
			DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
		)
		_, err = tx.Tx.ExecContext(ctx, statement,
			projectID[:], bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, 0, 0, uint64(settledAmount), uint64(settledAmount),
		)
		if err != nil {
			return ErrUpdateBucketBandwidthSettle.Wrap(err)
		}

		if action == pb.PieceAction_GET {
			dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)
			statement = tx.Rebind(
				`INSERT INTO project_bandwidth_daily_rollups (project_id, interval_day, egress_allocated, egress_settled, egress_dead)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(project_id, interval_day)
				DO UPDATE SET
					egress_settled = project_bandwidth_daily_rollups.egress_settled + EXCLUDED.egress_settled::BIGINT,
					egress_dead    = project_bandwidth_daily_rollups.egress_dead + EXCLUDED.egress_dead::BIGINT`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement, projectID[:], dailyInterval, 0, uint64(settledAmount), uint64(deadAmount))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := db.db.Rebind(
		`INSERT INTO bucket_bandwidth_rollups (project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, bucket_name, interval_start, action)
		DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
	)
	_, err = db.db.ExecContext(ctx, statement,
		projectID[:], bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount),
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
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE project_id = ? AND bucket_name = ? AND interval_start > ? AND interval_start <= ?`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), projectID[:], bucketName, from.UTC(), to.UTC()).Scan(&sum)
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

// UpdateBandwidthBatch updates bucket and project bandwidth rollups in the database.
func (db *ordersDB) UpdateBandwidthBatch(ctx context.Context, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(rollups) == 0 {
		return nil
	}

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		defer mon.Task()(&ctx)(&err)

		// TODO reorg code to make clear what we are inserting/updating to
		// bucket_bandwidth_rollups and project_bandwidth_daily_rollups

		bucketRUMap := rollupBandwidth(rollups, toHourlyInterval, getBucketRollupKey)

		projectIDs := make([]uuid.UUID, 0, len(bucketRUMap))
		bucketNames := make([][]byte, 0, len(bucketRUMap))
		intervalStartSlice := make([]time.Time, 0, len(bucketRUMap))
		actionSlice := make([]int32, 0, len(bucketRUMap))
		inlineSlice := make([]int64, 0, len(bucketRUMap))
		settledSlice := make([]int64, 0, len(bucketRUMap))

		bucketRUMapKeys := make([]bandwidthRollupKey, 0, len(bucketRUMap))
		for key := range bucketRUMap {
			bucketRUMapKeys = append(bucketRUMapKeys, key)
		}

		sortBandwidthRollupKeys(bucketRUMapKeys)

		for _, rollupInfo := range bucketRUMapKeys {
			usage := bucketRUMap[rollupInfo]
			if usage.Inline != 0 || usage.Settled != 0 {
				projectIDs = append(projectIDs, rollupInfo.ProjectID)
				bucketNames = append(bucketNames, []byte(rollupInfo.BucketName))
				intervalStartSlice = append(intervalStartSlice, time.Unix(rollupInfo.IntervalStart, 0))
				actionSlice = append(actionSlice, int32(rollupInfo.Action))
				inlineSlice = append(inlineSlice, usage.Inline)
				settledSlice = append(settledSlice, usage.Settled)
			}
		}

		// allocated must be not-null so lets keep slice until we will change DB schema
		emptyAllocatedSlice := make([]int64, len(projectIDs))

		if len(projectIDs) > 0 {
			_, err = tx.Tx.ExecContext(ctx, `
				INSERT INTO bucket_bandwidth_rollups (
					project_id, bucket_name,
					interval_start, interval_seconds,
					action, inline, allocated, settled)
				SELECT
					unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::timestamptz[]),
					$4,
					unnest($5::int4[]), unnest($6::bigint[]), unnest($7::bigint[]), unnest($8::bigint[])
				ON CONFLICT(project_id, bucket_name, interval_start, action)
				DO UPDATE SET
					inline = bucket_bandwidth_rollups.inline + EXCLUDED.inline,
					settled = bucket_bandwidth_rollups.settled + EXCLUDED.settled
			`, pgutil.UUIDArray(projectIDs), pgutil.ByteaArray(bucketNames), pgutil.TimestampTZArray(intervalStartSlice),
				defaultIntervalSeconds,
				pgutil.Int4Array(actionSlice), pgutil.Int8Array(inlineSlice), pgutil.Int8Array(emptyAllocatedSlice), pgutil.Int8Array(settledSlice))
			if err != nil {
				return errs.New("bucket bandwidth rollup batch flush failed: %+v", err)
			}
		}

		projectRUMap := rollupBandwidth(rollups, toDailyInterval, getProjectRollupKey)

		projectIDs = make([]uuid.UUID, 0, len(projectRUMap))
		intervalStartSlice = make([]time.Time, 0, len(projectRUMap))
		allocatedSlice := make([]int64, 0, len(projectRUMap))
		settledSlice = make([]int64, 0, len(projectRUMap))
		deadSlice := make([]int64, 0, len(projectRUMap))

		projectRUMapKeys := make([]bandwidthRollupKey, 0, len(projectRUMap))
		for key := range projectRUMap {
			if key.Action == pb.PieceAction_GET {
				projectRUMapKeys = append(projectRUMapKeys, key)
			}
		}

		sortBandwidthRollupKeys(projectRUMapKeys)

		for _, rollupInfo := range projectRUMapKeys {
			usage := projectRUMap[rollupInfo]
			projectIDs = append(projectIDs, rollupInfo.ProjectID)
			intervalStartSlice = append(intervalStartSlice, time.Unix(rollupInfo.IntervalStart, 0))

			allocatedSlice = append(allocatedSlice, usage.Allocated)
			settledSlice = append(settledSlice, usage.Settled)
			deadSlice = append(deadSlice, usage.Dead)
		}

		if len(projectIDs) > 0 {
			// TODO: explore updating project_bandwidth_daily_rollups table to use "timestamp with time zone" for interval_day
			_, err = tx.Tx.ExecContext(ctx, `
				INSERT INTO project_bandwidth_daily_rollups(project_id, interval_day, egress_allocated, egress_settled, egress_dead)
					SELECT unnest($1::bytea[]), unnest($2::date[]), unnest($3::bigint[]), unnest($4::bigint[]), unnest($5::bigint[])
				ON CONFLICT(project_id, interval_day)
				DO UPDATE SET
					egress_allocated = project_bandwidth_daily_rollups.egress_allocated + EXCLUDED.egress_allocated::bigint,
					egress_settled   = project_bandwidth_daily_rollups.egress_settled   + EXCLUDED.egress_settled::bigint,
					egress_dead      = project_bandwidth_daily_rollups.egress_dead      + EXCLUDED.egress_dead::bigint
			`, pgutil.UUIDArray(projectIDs), pgutil.DateArray(intervalStartSlice), pgutil.Int8Array(allocatedSlice), pgutil.Int8Array(settledSlice), pgutil.Int8Array(deadSlice))
			if err != nil {
				return errs.New("project bandwidth daily rollup batch flush failed: %+v", err)
			}
		}
		return nil
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
		if dbx.IsConstraintError(err) {
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

// toDailyInterval rounds the time stamp down to the start of the day and converts it to unix time.
func toDailyInterval(timeInterval time.Time) int64 {
	return time.Date(timeInterval.Year(), timeInterval.Month(), timeInterval.Day(), 0, 0, 0, 0, timeInterval.Location()).Unix()
}

// toHourlyInterval rounds the time stamp down to the start of the hour and converts it to unix time.
func toHourlyInterval(timeInterval time.Time) int64 {
	return time.Date(timeInterval.Year(), timeInterval.Month(), timeInterval.Day(), timeInterval.Hour(), 0, 0, 0, timeInterval.Location()).Unix()
}

// rollupBandwidth rollup the bandwidth statistics into a map based on the provided key, interval.
func rollupBandwidth(rollups []orders.BucketBandwidthRollup,
	toInterval func(time.Time) int64,
	getKey func(orders.BucketBandwidthRollup, func(time.Time) int64) bandwidthRollupKey) map[bandwidthRollupKey]bandwidth {
	projectRUMap := make(map[bandwidthRollupKey]bandwidth)

	for _, rollup := range rollups {
		rollup := rollup
		projectKey := getKey(rollup, toInterval)
		if b, ok := projectRUMap[projectKey]; ok {
			b.Allocated += rollup.Allocated
			b.Settled += rollup.Settled
			b.Inline += rollup.Inline
			b.Dead += rollup.Dead
			projectRUMap[projectKey] = b
		} else {
			projectRUMap[projectKey] = bandwidth{
				Allocated: rollup.Allocated,
				Settled:   rollup.Settled,
				Inline:    rollup.Inline,
				Dead:      rollup.Dead,
			}
		}
	}

	return projectRUMap
}

// getBucketRollupKey return a key for use in bucket bandwidth rollup statistics.
func getBucketRollupKey(rollup orders.BucketBandwidthRollup, toInterval func(time.Time) int64) bandwidthRollupKey {
	return bandwidthRollupKey{
		BucketName:    rollup.BucketName,
		ProjectID:     rollup.ProjectID,
		IntervalStart: toInterval(rollup.IntervalStart),
		Action:        rollup.Action,
	}
}

// getProjectRollupKey return a key for use in project bandwidth rollup statistics.
func getProjectRollupKey(rollup orders.BucketBandwidthRollup, toInterval func(time.Time) int64) bandwidthRollupKey {
	return bandwidthRollupKey{
		ProjectID:     rollup.ProjectID,
		IntervalStart: toInterval(rollup.IntervalStart),
		Action:        rollup.Action,
	}
}

func sortBandwidthRollupKeys(bandwidthRollupKeys []bandwidthRollupKey) {
	sort.SliceStable(bandwidthRollupKeys, func(i, j int) bool {
		uuidCompare := bandwidthRollupKeys[i].ProjectID.Compare(bandwidthRollupKeys[j].ProjectID)
		switch {
		case uuidCompare == -1:
			return true
		case uuidCompare == 1:
			return false
		case bandwidthRollupKeys[i].BucketName < bandwidthRollupKeys[j].BucketName:
			return true
		case bandwidthRollupKeys[i].BucketName > bandwidthRollupKeys[j].BucketName:
			return false
		case bandwidthRollupKeys[i].IntervalStart < bandwidthRollupKeys[j].IntervalStart:
			return true
		case bandwidthRollupKeys[i].IntervalStart > bandwidthRollupKeys[j].IntervalStart:
			return false
		case bandwidthRollupKeys[i].Action < bandwidthRollupKeys[j].Action:
			return true
		case bandwidthRollupKeys[i].Action > bandwidthRollupKeys[j].Action:
			return false
		default:
			return false
		}
	})
}
