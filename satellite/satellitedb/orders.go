// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/spanner"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"golang.org/x/exp/maps"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/dbutil/spannerutil"
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
	db             *satelliteDB
	maxCommitDelay *time.Duration
}

type bandwidth struct {
	Allocated int64
	Settled   int64
	Inline    int64
	Dead      int64
}

// BandwidthRollupKey is used to collect data for a query.
type BandwidthRollupKey struct {
	BucketName    string
	ProjectID     uuid.UUID
	IntervalStart int64
	Action        pb.PieceAction
}

// BucketBandwidthRollup is a type to encapsulate the values to insert into a record
// for the bucket_bandwidth_rollups table.
type BucketBandwidthRollup struct {
	BucketName      []byte
	ProjectID       uuid.UUID
	IntervalStart   time.Time
	IntervalSeconds int64
	Action          int64
	Inline          int64
	Allocated       int64
	Settled         int64
}

// ProjectBandwidthDailyRollup is a type to encapsulate the values to insert into a record
// for the project_bandwidth_daily_rollups table.
type ProjectBandwidthDailyRollup struct {
	ProjectID       uuid.UUID
	IntervalStart   civil.Date
	EgressAllocated int64
	EgressSettled   int64
	EgressDead      int64
}

// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
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
			batch.Queue(statement, projectID, bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, 0, uint64(amount), 0, uint64(amount))

			if action == pb.PieceAction_GET {
				statement = db.db.Rebind(
					`INSERT INTO project_bandwidth_daily_rollups (project_id, interval_day, egress_allocated, egress_settled, egress_dead)
					VALUES (?, ?, ?, ?, ?)
					ON CONFLICT(project_id, interval_day)
					DO UPDATE SET egress_allocated = project_bandwidth_daily_rollups.egress_allocated + EXCLUDED.egress_allocated::BIGINT`,
				)
				batch.Queue(statement, projectID, dailyInterval, uint64(amount), 0, 0)
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
	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
			defer mon.Task()(&ctx)(&err)

			dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)
			civilDailyIntervalDate := civil.DateOf(dailyInterval)

			// Spanner does not support `INSERT INTO ... ON CONFLICT DO UPDATE SET`, see details in [doc.go].

			statements := []spanner.Statement{
				{
					SQL: `
						UPDATE bucket_bandwidth_rollups
						SET allocated = allocated + @amount
						WHERE (project_id, bucket_name, interval_start, action) = (@project_id, @bucket_name, @interval_start, @action)
					`,
					Params: map[string]any{
						"amount":         amount,
						"project_id":     projectID.Bytes(),
						"bucket_name":    bucketName,
						"interval_start": intervalStart,
						"action":         int64(action),
					},
				},
				{
					SQL: `
						INSERT OR IGNORE INTO bucket_bandwidth_rollups
							(project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
						VALUES (@project_id, @bucket_name, @interval_start, @interval_seconds, @action, 0, @amount, 0)
					`,
					Params: map[string]any{
						"project_id":       projectID.Bytes(),
						"bucket_name":      bucketName,
						"interval_start":   intervalStart,
						"interval_seconds": defaultIntervalSeconds,
						"action":           int64(action),
						"amount":           amount,
					},
				},
			}

			if action == pb.PieceAction_GET {
				statements = append(statements,
					spanner.Statement{
						SQL: `
							UPDATE project_bandwidth_daily_rollups
							SET egress_allocated = egress_allocated + @amount
							WHERE (project_id, interval_day) = (@project_id, @interval_day)
						`,
						Params: map[string]any{
							"amount":       amount,
							"project_id":   projectID.Bytes(),
							"interval_day": civilDailyIntervalDate,
						},
					},
					spanner.Statement{
						SQL: `
							INSERT OR IGNORE INTO project_bandwidth_daily_rollups
								(project_id, interval_day, egress_allocated, egress_settled, egress_dead)
							VALUES (@project_id, @interval_day, @amount, 0, 0)
						`,
						Params: map[string]any{
							"project_id":   projectID.Bytes(),
							"interval_day": civilDailyIntervalDate,
							"amount":       amount,
						},
					},
				)
			}

			_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
				_, err := txn.BatchUpdateWithOptions(ctx, statements, spanner.QueryOptions{
					RequestTag: "orders/update-bucket-bandwidth-allocation",
				})
				return err
			}, spanner.TransactionOptions{
				TransactionTag: "orders/update-bucket-bandwidth-allocation",
			})
			return errs.Wrap(err)
		})
	default:
		return errs.Wrap(fmt.Errorf("unsupported database dialect: %s", db.db.impl))
	}
}

// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, settledAmount, deadAmount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			statement := tx.Rebind(
				`INSERT INTO bucket_bandwidth_rollups (project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (project_id, bucket_name, interval_start, action)
				DO UPDATE SET settled = bucket_bandwidth_rollups.settled + ?`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement,
				projectID, bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, 0, 0, uint64(settledAmount), uint64(settledAmount),
			)
			if err != nil {
				return ErrUpdateBucketBandwidthSettle.Wrap(err)
			}

			if action == pb.PieceAction_GET {
				dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)
				statement = tx.Rebind(
					`INSERT INTO project_bandwidth_daily_rollups (project_id, interval_day, egress_allocated, egress_settled, egress_dead)
					VALUES (?, ?, ?, ?, ?)
					ON CONFLICT (project_id, interval_day)
					DO UPDATE SET
						egress_settled = project_bandwidth_daily_rollups.egress_settled + EXCLUDED.egress_settled::BIGINT,
						egress_dead    = project_bandwidth_daily_rollups.egress_dead + EXCLUDED.egress_dead::BIGINT`,
				)
				_, err = tx.Tx.ExecContext(ctx, statement, projectID, dailyInterval, 0, uint64(settledAmount), uint64(deadAmount))
				if err != nil {
					return err
				}
			}
			return nil
		})
	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
			defer mon.Task()(&ctx)(&err)

			dailyInterval := time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 0, 0, 0, 0, time.UTC)
			civilDailyIntervalDate := civil.DateOf(dailyInterval)

			// Spanner does not support `INSERT INTO ... ON CONFLICT DO UPDATE SET`, see details in [doc.go].

			statements := []spanner.Statement{
				{
					SQL: `
						UPDATE bucket_bandwidth_rollups
						SET settled = settled + @settled_amount
						WHERE (project_id, bucket_name, interval_start, action) = (@project_id, @bucket_name, @interval_start, @action)
					`,
					Params: map[string]any{
						"settled_amount": settledAmount,
						"project_id":     projectID.Bytes(),
						"bucket_name":    bucketName,
						"interval_start": intervalStart,
						"action":         int64(action),
					},
				},
				{
					SQL: `
						INSERT OR IGNORE INTO bucket_bandwidth_rollups
							(project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
						VALUES (@project_id, @bucket_name, @interval_start, @interval_seconds, @action, 0, 0, @settled_amount)
					`,
					Params: map[string]any{
						"project_id":       projectID.Bytes(),
						"bucket_name":      bucketName,
						"interval_start":   intervalStart,
						"interval_seconds": defaultIntervalSeconds,
						"action":           int64(action),
						"settled_amount":   settledAmount,
					},
				},
			}

			if action == pb.PieceAction_GET {
				statements = append(statements,
					spanner.Statement{
						SQL: `
							UPDATE project_bandwidth_daily_rollups
							SET egress_settled = egress_settled + @settled_amount,
							egress_dead = egress_dead + @dead_amount
						WHERE
							(project_id, interval_day) = (@project_id, @interval_day)
						`,
						Params: map[string]any{
							"settled_amount": settledAmount,
							"dead_amount":    deadAmount,
							"project_id":     projectID.Bytes(),
							"interval_day":   civilDailyIntervalDate,
						},
					},
					spanner.Statement{
						SQL: `
							INSERT OR IGNORE INTO project_bandwidth_daily_rollups
								(project_id, interval_day, egress_allocated, egress_settled, egress_dead)
							VALUES (@project_id, @interval_day, 0, @settled_amount, @dead_amount)
						`,
						Params: map[string]any{
							"project_id":     projectID.Bytes(),
							"interval_day":   civilDailyIntervalDate,
							"settled_amount": settledAmount,
							"dead_amount":    deadAmount,
						},
					},
				)
			}

			_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
				_, err := txn.BatchUpdate(ctx, statements)
				return err
			}, spanner.TransactionOptions{
				TransactionTag: "orders/update-bucket-bandwidth-settle",
			})
			return errs.Wrap(err)
		})
	default:
		return ErrUpdateBucketBandwidthSettle.New("unsupported database dialect: %s", db.db.impl)
	}
}

// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket.
func (db *ordersDB) UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		statement := db.db.Rebind(
			`INSERT INTO bucket_bandwidth_rollups (project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(project_id, bucket_name, interval_start, action)
			DO UPDATE SET inline = bucket_bandwidth_rollups.inline + ?`,
		)
		_, err = db.db.ExecContext(ctx, statement,
			projectID, bucketName, intervalStart.UTC(), defaultIntervalSeconds, action, uint64(amount), 0, 0, uint64(amount),
		)
		if err != nil {
			return errs.Wrap(err)
		}
		return nil
	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
			defer mon.Task()(&ctx)(&err)

			// Construct statements for bucket_bandwidth_rollups
			statements := []spanner.Statement{
				{
					SQL: `
						UPDATE bucket_bandwidth_rollups
						SET inline = inline + @amount
						WHERE (project_id, bucket_name, interval_start, action) = (@project_id, @bucket_name, @interval_start, @action)
					`,
					Params: map[string]any{
						"amount":         amount,
						"project_id":     projectID.Bytes(),
						"bucket_name":    bucketName,
						"interval_start": intervalStart,
						"action":         int64(action),
					},
				},
				{
					SQL: `
						INSERT OR IGNORE INTO bucket_bandwidth_rollups
							(project_id, bucket_name, interval_start, interval_seconds, action, inline, allocated, settled)
						VALUES (@project_id, @bucket_name, @interval_start, @interval_seconds, @action, @amount, 0, 0)
					`,
					Params: map[string]any{
						"project_id":       projectID.Bytes(),
						"bucket_name":      bucketName,
						"interval_start":   intervalStart,
						"interval_seconds": defaultIntervalSeconds,
						"action":           int64(action),
						"amount":           amount,
					},
				},
			}

			_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
				_, err := txn.BatchUpdate(ctx, statements)
				return err
			}, spanner.TransactionOptions{
				TransactionTag: "orders/update-bucket-bandwidth-inline",
			})
			return errs.Wrap(err)
		})
	default:
		return errs.New("unsupported database dialect: %s", db.db.impl)
	}
}

// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node for the given intervalStart time.
func (db *ordersDB) UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		statement := db.db.Rebind(
			`INSERT INTO storagenode_bandwidth_rollups (storagenode_id, interval_start, interval_seconds, action, settled)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(storagenode_id, interval_start, action)
			DO UPDATE SET settled = storagenode_bandwidth_rollups.settled + ?`,
		)
		_, err = db.db.ExecContext(ctx, statement,
			storageNode, intervalStart.UTC(), defaultIntervalSeconds, action, uint64(amount), uint64(amount),
		)
		if err != nil {
			return err
		}
		return nil
	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
			defer mon.Task()(&ctx)(&err)

			// Construct statements for storagenode_bandwidth_rollups
			statements := []spanner.Statement{
				{
					SQL: `
						UPDATE storagenode_bandwidth_rollups
						SET settled = settled + @amount
						WHERE (storagenode_id, interval_start, action) = (@storagenode_id, @interval_start, @action)
					`,
					Params: map[string]any{
						"amount":         amount,
						"storagenode_id": storageNode.Bytes(),
						"interval_start": intervalStart,
						"action":         int64(action),
					},
				},
				{
					SQL: `
						INSERT OR IGNORE INTO storagenode_bandwidth_rollups
							(storagenode_id, interval_start, interval_seconds, action, settled)
						VALUES (@storagenode_id, @interval_start, @interval_seconds, @action, @amount)
					`,
					Params: map[string]any{
						"storagenode_id":   storageNode.Bytes(),
						"interval_start":   intervalStart,
						"interval_seconds": defaultIntervalSeconds,
						"action":           int64(action),
						"amount":           amount,
					},
				},
			}

			_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
				_, err := txn.BatchUpdate(ctx, statements)
				return err
			}, spanner.TransactionOptions{
				TransactionTag: "orders/update-storagenode-bandwidth-settle",
			})
			return errs.Wrap(err)
		})
	default:
		return errs.New("unsupported database dialect: %s", db.db.impl)
	}
}

// TestGetBucketBandwidth gets total bucket bandwidth (allocated,inline,settled).
func (db *ordersDB) TestGetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (allocated int64, inline int64, settled int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT SUM(allocated),SUM(inline), SUM(settled) FROM bucket_bandwidth_rollups WHERE project_id = ? AND bucket_name = ? AND interval_start > ? AND interval_start <= ?`

	var (
		a sql.NullInt64
		i sql.NullInt64
		s sql.NullInt64
	)
	err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID, bucketName, from.UTC(), to.UTC()).Scan(&a, &i, &s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, nil
		}

		return 0, 0, 0, Error.Wrap(err)
	}

	if a.Valid {
		allocated = a.Int64
	}
	if i.Valid {
		inline = i.Int64
	}
	if s.Valid {
		settled = s.Int64
	}
	return allocated, inline, settled, nil
}

// GetStorageNodeBandwidth gets total storage node bandwidth from period of time.
func (db *ordersDB) GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var sum int64

	err = db.db.QueryRowContext(ctx, db.db.Rebind(`
		SELECT COALESCE(SUM(settled), 0)
		FROM storagenode_bandwidth_rollups
		WHERE storagenode_id = ?
			AND interval_start > ?
			AND interval_start <= ?
	`), nodeID, from.UTC(), to.UTC()).Scan(&sum)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	return sum, nil
}

// UpdateBandwidthBatch updates bucket and project bandwidth rollups in the database.
func (db *ordersDB) UpdateBandwidthBatch(ctx context.Context, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(rollups) == 0 {
		return nil
	}

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		return db.updateBandwidthBatchPostgres(ctx, rollups)
	case dbutil.Spanner:
		return db.updateBandwidthBatchSpanner(ctx, rollups)
	default:
		return errs.New("unsupported database dialect: %s", db.db.impl)
	}
}

// updateBandwidthBatchPostgres updates bucket and project bandwidth rollups in the database.
func (db *ordersDB) updateBandwidthBatchPostgres(ctx context.Context, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		defer mon.Task()(&ctx)(&err)

		var (
			bucketUpdates  = int(0)
			projectUpdates = int(0)
		)
		defer func() {
			if err == nil {
				mon.Meter("update_bandwidth_batch_bucket_items_successful").Mark(bucketUpdates)
				mon.Meter("update_bandwidth_batch_project_items_successful").Mark(projectUpdates)
			}
		}()

		// TODO reorg code to make clear what we are inserting/updating to
		// bucket_bandwidth_rollups and project_bandwidth_daily_rollups

		bucketRUMap := rollupBandwidth(rollups, toHourlyInterval, getBucketRollupKey)

		projectIDs := make([]uuid.UUID, 0, len(bucketRUMap))
		bucketNames := make([][]byte, 0, len(bucketRUMap))
		intervalStartSlice := make([]time.Time, 0, len(bucketRUMap))
		actionSlice := make([]int32, 0, len(bucketRUMap))
		inlineSlice := make([]int64, 0, len(bucketRUMap))
		settledSlice := make([]int64, 0, len(bucketRUMap))

		bucketRUMapKeys := make([]BandwidthRollupKey, 0, len(bucketRUMap))
		for key := range bucketRUMap {
			bucketRUMapKeys = append(bucketRUMapKeys, key)
		}

		SortBandwidthRollupKeys(bucketRUMapKeys)

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

		bucketUpdates = len(projectIDs)
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
				return errs.New("bucket bandwidth rollup batch flush failed: %w", err)
			}
		}

		projectRUMap := rollupBandwidth(rollups, toDailyInterval, getProjectRollupKey)

		projectIDs = make([]uuid.UUID, 0, len(projectRUMap))
		intervalStartSlice = make([]time.Time, 0, len(projectRUMap))
		allocatedSlice := make([]int64, 0, len(projectRUMap))
		settledSlice = make([]int64, 0, len(projectRUMap))
		deadSlice := make([]int64, 0, len(projectRUMap))

		projectRUMapKeys := make([]BandwidthRollupKey, 0, len(projectRUMap))
		for key := range projectRUMap {
			if key.Action == pb.PieceAction_GET {
				projectRUMapKeys = append(projectRUMapKeys, key)
			}
		}

		SortBandwidthRollupKeys(projectRUMapKeys)

		for _, rollupInfo := range projectRUMapKeys {
			usage := projectRUMap[rollupInfo]
			projectIDs = append(projectIDs, rollupInfo.ProjectID)
			intervalStartSlice = append(intervalStartSlice, time.Unix(rollupInfo.IntervalStart, 0))

			allocatedSlice = append(allocatedSlice, usage.Allocated)
			settledSlice = append(settledSlice, usage.Settled)
			deadSlice = append(deadSlice, usage.Dead)
		}

		projectUpdates = len(projectIDs)
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
				return errs.New("project bandwidth daily rollup batch flush failed: %w", err)
			}
		}
		return nil
	})
}

// updateBandwidthBatchSpanner updates bucket and project bandwidth rollups in the database.
func (db *ordersDB) updateBandwidthBatchSpanner(ctx context.Context, rollups []orders.BucketBandwidthRollup) (err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		bucketUpdates  = int(0)
		projectUpdates = int(0)
	)
	defer func() {
		if err == nil {
			mon.Meter("update_bandwidth_batch_bucket_items_successful").Mark(bucketUpdates)
			mon.Meter("update_bandwidth_batch_project_items_successful").Mark(projectUpdates)
		}
	}()

	statements := []spanner.Statement{}

	// Spanner does not support `INSERT INTO ... ON CONFLICT DO UPDATE SET`, see details in [doc.go].

	{ // construct bucket_bandwidth_rollups statements
		bucketRollupMap := rollupBandwidth(rollups, toHourlyInterval, getBucketRollupKey)

		bucketRollupKeys := maps.Keys(bucketRollupMap)
		SortBandwidthRollupKeys(bucketRollupKeys)

		type update struct {
			ProjectID       []byte
			BucketName      []byte
			IntervalStart   time.Time
			IntervalSeconds int64
			Action          int64
			Inline          int64
			Settled         int64
		}

		updates := make([]update, 0, len(bucketRollupKeys))

		for _, key := range bucketRollupKeys {
			usage := bucketRollupMap[key]
			if usage.Inline == 0 && usage.Settled == 0 {
				continue
			}

			updates = append(updates, update{
				ProjectID:       key.ProjectID.Bytes(),
				BucketName:      []byte(key.BucketName),
				IntervalStart:   time.Unix(key.IntervalStart, 0),
				IntervalSeconds: int64(defaultIntervalSeconds),
				Action:          int64(key.Action),
				Inline:          usage.Inline,
				Settled:         usage.Settled,
			})
		}

		bucketUpdates = len(updates)
		if len(updates) > 0 {
			for i := range updates {
				up := &updates[i]

				statements = append(statements, spanner.Statement{
					SQL: `
						UPDATE bucket_bandwidth_rollups bbr
						SET
							inline = inline + @inline,
							settled = settled + @settled
						WHERE (project_id, bucket_name, interval_start, action) =
							(@project_id, @bucket_name, @interval_start, @action)
					`,
					Params: map[string]any{
						"project_id":       up.ProjectID,
						"bucket_name":      up.BucketName,
						"interval_start":   up.IntervalStart,
						"interval_seconds": up.IntervalSeconds,
						"action":           up.Action,
						"inline":           up.Inline,
						"settled":          up.Settled,
					},
				})
			}

			statements = append(statements, spanner.Statement{
				SQL: `
					INSERT OR IGNORE INTO bucket_bandwidth_rollups (
						project_id, bucket_name,
						interval_start, interval_seconds,
						action, inline, allocated, settled
					)
					(SELECT ProjectID, BucketName, IntervalStart, IntervalSeconds, Action, Inline, 0, Settled FROM UNNEST(@updates))
				`,
				Params: map[string]any{
					"updates": updates,
				},
			})
		}
	}

	{ // construct project_bandwidth_daily_rollups statements
		projectRollupsMap := rollupBandwidth(rollups, toDailyInterval, getProjectRollupKey)

		projectRollupKeys := make([]BandwidthRollupKey, 0, len(projectRollupsMap))
		for key := range projectRollupsMap {
			if key.Action == pb.PieceAction_GET {
				projectRollupKeys = append(projectRollupKeys, key)
			}
		}

		SortBandwidthRollupKeys(projectRollupKeys)

		type update struct {
			ProjectID       []byte
			IntervalDay     civil.Date
			EgressAllocated int64
			EgressSettled   int64
			EgressDead      int64
		}

		updates := make([]update, 0, len(projectRollupKeys))

		for _, key := range projectRollupKeys {
			usage := projectRollupsMap[key]
			updates = append(updates, update{
				ProjectID:       key.ProjectID.Bytes(),
				IntervalDay:     civil.DateOf(time.Unix(key.IntervalStart, 0)),
				EgressAllocated: usage.Allocated,
				EgressSettled:   usage.Settled,
				EgressDead:      usage.Dead,
			})
		}

		projectUpdates = len(updates)
		if len(updates) > 0 {
			for i := range updates {
				up := &updates[i]

				statements = append(statements, spanner.Statement{
					SQL: `
						UPDATE project_bandwidth_daily_rollups
						SET
							egress_allocated = egress_allocated + @egress_allocated,
							egress_settled = egress_settled + @egress_settled,
							egress_dead = egress_dead + @egress_dead
						WHERE
							(project_id, interval_day) = (@project_id, @interval_day)
					`,
					Params: map[string]any{
						"project_id":       up.ProjectID,
						"interval_day":     up.IntervalDay,
						"egress_allocated": up.EgressAllocated,
						"egress_settled":   up.EgressSettled,
						"egress_dead":      up.EgressDead,
					},
				})
			}

			statements = append(statements, spanner.Statement{
				SQL: `
					INSERT OR IGNORE INTO project_bandwidth_daily_rollups (
						project_id, interval_day,
						egress_allocated, egress_settled, egress_dead
					)
					(SELECT ProjectID, IntervalDay, EgressAllocated, EgressSettled, EgressDead FROM UNNEST(@updates))
				`,
				Params: map[string]any{
					"updates": updates,
				},
			})
		}
	}

	if len(statements) == 0 {
		return nil
	}

	return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
		defer mon.Task()(&ctx)(&err)

		_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
			_, err := txn.BatchUpdate(ctx, statements)
			if err != nil {
				return Error.New("failed to into bucket_bandwidth_rollups and project_bandwidth_daily_rollups tables: %w", err)
			}
			return nil
		}, spanner.TransactionOptions{
			CommitOptions: spanner.CommitOptions{
				MaxCommitDelay: db.maxCommitDelay,
			},
		})
		return Error.Wrap(err)
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
	getKey func(orders.BucketBandwidthRollup, func(time.Time) int64) BandwidthRollupKey) map[BandwidthRollupKey]bandwidth {
	projectRUMap := make(map[BandwidthRollupKey]bandwidth)

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
func getBucketRollupKey(rollup orders.BucketBandwidthRollup, toInterval func(time.Time) int64) BandwidthRollupKey {
	return BandwidthRollupKey{
		BucketName:    rollup.BucketName,
		ProjectID:     rollup.ProjectID,
		IntervalStart: toInterval(rollup.IntervalStart),
		Action:        rollup.Action,
	}
}

// getProjectRollupKey return a key for use in project bandwidth rollup statistics.
func getProjectRollupKey(rollup orders.BucketBandwidthRollup, toInterval func(time.Time) int64) BandwidthRollupKey {
	return BandwidthRollupKey{
		ProjectID:     rollup.ProjectID,
		IntervalStart: toInterval(rollup.IntervalStart),
		Action:        rollup.Action,
	}
}

// SortBandwidthRollupKeys sorts bandwidth rollups.
func SortBandwidthRollupKeys(bandwidthRollupKeys []BandwidthRollupKey) {
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
