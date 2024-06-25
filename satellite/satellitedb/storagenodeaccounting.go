// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/cockroachutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

// StoragenodeAccounting implements the accounting/db StoragenodeAccounting interface.
type StoragenodeAccounting struct {
	db *satelliteDB
}

// SaveTallies records raw tallies of at rest data to the database.
func (db *StoragenodeAccounting) SaveTallies(ctx context.Context, latestTally time.Time, nodeIDs []storj.NodeID, totals []float64) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		_, err = tx.Tx.ExecContext(ctx, db.db.Rebind(`
			INSERT INTO storagenode_storage_tallies (
				interval_end_time,
				node_id, data_total)
			SELECT
				$1,
				unnest($2::bytea[]), unnest($3::float8[])`),
			latestTally,
			pgutil.NodeIDArray(nodeIDs), pgutil.Float8Array(totals))
		if err != nil {
			return err
		}
		return tx.ReplaceNoReturn_AccountingTimestamps(ctx,
			dbx.AccountingTimestamps_Name(accounting.LastAtRestTally),
			dbx.AccountingTimestamps_Value(latestTally),
		)
	})
	return Error.Wrap(err)
}

// GetTallies retrieves all raw tallies.
func (db *StoragenodeAccounting) GetTallies(ctx context.Context) (_ []*accounting.StoragenodeStorageTally, err error) {
	defer mon.Task()(&ctx)(&err)
	raws, err := db.db.All_StoragenodeStorageTally(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	out, err := slices2.Convert(raws, fromDBXStoragenodeStorageTally)
	return out, Error.Wrap(err)
}

// GetTalliesSince retrieves all raw tallies since latestRollup.
func (db *StoragenodeAccounting) GetTalliesSince(ctx context.Context, latestRollup time.Time) (_ []*accounting.StoragenodeStorageTally, err error) {
	defer mon.Task()(&ctx)(&err)
	raws, err := db.db.All_StoragenodeStorageTally_By_IntervalEndTime_GreaterOrEqual(ctx, dbx.StoragenodeStorageTally_IntervalEndTime(latestRollup))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	out, err := slices2.Convert(raws, fromDBXStoragenodeStorageTally)
	return out, Error.Wrap(err)
}

func (db *StoragenodeAccounting) getNodeIdsSince(ctx context.Context, since time.Time) (nodeids [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	rows, err := db.db.QueryContext(ctx, db.db.Rebind(`SELECT DISTINCT storagenode_id FROM storagenode_bandwidth_rollups WHERE interval_start >= $1`), since)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(rows.Close()))
	}()

	for rows.Next() {
		var nodeid []byte
		err := rows.Scan(&nodeid)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		nodeids = append(nodeids, nodeid)
	}
	err = rows.Err()
	if err != nil {
		return nil, Error.Wrap(rows.Err())
	}

	return nodeids, nil
}

func (db *StoragenodeAccounting) getBandwidthByNodeSince(ctx context.Context, latestRollup time.Time, nodeid []byte,
	cb func(context.Context, *accounting.StoragenodeBandwidthRollup) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_StoragenodeBandwidthRollup_By_StoragenodeId_And_IntervalStart_GreaterOrEqual_Continuation
	for {
		rollups, next, err := db.db.Paged_StoragenodeBandwidthRollup_By_StoragenodeId_And_IntervalStart_GreaterOrEqual(ctx,
			dbx.StoragenodeBandwidthRollup_StoragenodeId(nodeid), dbx.StoragenodeBandwidthRollup_IntervalStart(latestRollup),
			pageLimit, cursor)
		if err != nil {
			return Error.Wrap(err)
		}
		cursor = next
		for _, r := range rollups {
			v, err := fromDBXStoragenodeBandwidthRollup(r)
			if err != nil {
				return err
			}
			err = cb(ctx, &v)
			if err != nil {
				return err
			}
		}
		if cursor == nil {
			return nil
		}
	}
}

func (db *StoragenodeAccounting) getBandwidthPhase2ByNodeSince(ctx context.Context, latestRollup time.Time, nodeid []byte,
	cb func(context.Context, *accounting.StoragenodeBandwidthRollup) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_StoragenodeBandwidthRollupPhase2_By_StoragenodeId_And_IntervalStart_GreaterOrEqual_Continuation
	for {
		rollups, next, err := db.db.Paged_StoragenodeBandwidthRollupPhase2_By_StoragenodeId_And_IntervalStart_GreaterOrEqual(ctx,
			dbx.StoragenodeBandwidthRollupPhase2_StoragenodeId(nodeid), dbx.StoragenodeBandwidthRollupPhase2_IntervalStart(latestRollup),
			pageLimit, cursor)
		if err != nil {
			return Error.Wrap(err)
		}
		cursor = next
		for _, r := range rollups {
			v, err := fromDBXStoragenodeBandwidthRollupPhase2(r)
			if err != nil {
				return Error.Wrap(err)
			}
			err = cb(ctx, &v)
			if err != nil {
				return err
			}
		}
		if cursor == nil {
			return nil
		}
	}
}

// GetBandwidthSince retrieves all storagenode_bandwidth_rollup entires since latestRollup.
func (db *StoragenodeAccounting) GetBandwidthSince(ctx context.Context, latestRollup time.Time,
	cb func(context.Context, *accounting.StoragenodeBandwidthRollup) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	// This table's key structure is storagenode_id, interval_start, so we're going to try and make
	// things easier on the database by making individual requests node by node. This is also
	// going to allow us to avoid 16 minute queries.
	var nodeids [][]byte
	for {
		nodeids, err = db.getNodeIdsSince(ctx, latestRollup)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return err
		}
		break
	}

	for _, nodeid := range nodeids {
		err = db.getBandwidthByNodeSince(ctx, latestRollup, nodeid, cb)
		if err != nil {
			return err
		}

		err = db.getBandwidthPhase2ByNodeSince(ctx, latestRollup, nodeid, cb)
		if err != nil {
			return err
		}
	}

	return nil

}

// SaveRollup records raw tallies of at rest data to the database.
func (db *StoragenodeAccounting) SaveRollup(ctx context.Context, latestRollup time.Time, stats accounting.RollupStats) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(stats) == 0 {
		return Error.New("In SaveRollup with empty nodeData")
	}

	batchSize := db.db.opts.SaveRollupBatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	var rollups []*accounting.Rollup
	for _, arsByDate := range stats {
		for _, ar := range arsByDate {
			rollups = append(rollups, ar)
		}
	}

	insertBatch := func(ctx context.Context, db *dbx.DB, batch []*accounting.Rollup) (err error) {
		defer mon.Task()(&ctx)(&err)
		n := len(batch)

		nodeID := make([]storj.NodeID, n)
		startTime := make([]time.Time, n)
		putTotal := make([]int64, n)
		getTotal := make([]int64, n)
		getAuditTotal := make([]int64, n)
		getRepairTotal := make([]int64, n)
		putRepairTotal := make([]int64, n)
		atRestTotal := make([]float64, n)
		intervalEndTime := make([]time.Time, n)

		for i, ar := range batch {
			nodeID[i] = ar.NodeID
			startTime[i] = ar.StartTime
			putTotal[i] = ar.PutTotal
			getTotal[i] = ar.GetTotal
			getAuditTotal[i] = ar.GetAuditTotal
			getRepairTotal[i] = ar.GetRepairTotal
			putRepairTotal[i] = ar.PutRepairTotal
			atRestTotal[i] = ar.AtRestTotal
			intervalEndTime[i] = ar.IntervalEndTime
		}

		_, err = db.ExecContext(ctx, `
			INSERT INTO accounting_rollups (
				node_id, start_time,
				put_total, get_total,
				get_audit_total, get_repair_total, put_repair_total,
				at_rest_total,
				interval_end_time
			)
			SELECT * FROM unnest(
				$1::bytea[], $2::timestamptz[],
				$3::int8[], $4::int8[],
				$5::int8[], $6::int8[], $7::int8[],
				$8::float8[],
				$9::timestamptz[]
			)
			ON CONFLICT ( node_id, start_time )
			DO UPDATE SET
				put_total = EXCLUDED.put_total,
				get_total = EXCLUDED.get_total,
				get_audit_total = EXCLUDED.get_audit_total,
				get_repair_total = EXCLUDED.get_repair_total,
				put_repair_total = EXCLUDED.put_repair_total,
				at_rest_total = EXCLUDED.at_rest_total,
				interval_end_time = EXCLUDED.interval_end_time
		`, pgutil.NodeIDArray(nodeID), pgutil.TimestampTZArray(startTime),
			pgutil.Int8Array(putTotal), pgutil.Int8Array(getTotal),
			pgutil.Int8Array(getAuditTotal), pgutil.Int8Array(getRepairTotal), pgutil.Int8Array(putRepairTotal),
			pgutil.Float8Array(atRestTotal),
			pgutil.TimestampTZArray(intervalEndTime))

		return Error.Wrap(err)
	}

	// Note: we do not need here a transaction because we will "update" the
	// columns when we do not update accounting.LastRollup. We will end up
	// with partial data in the database, however in the next runs, we will
	// try to fix them.

	for len(rollups) > 0 {
		batch := rollups
		if len(batch) > batchSize {
			batch = batch[:batchSize]
		}
		rollups = rollups[len(batch):]

		if err := insertBatch(ctx, db.db.DB, batch); err != nil {
			return Error.Wrap(err)
		}
	}

	err = db.db.UpdateNoReturn_AccountingTimestamps_By_Name(ctx,
		dbx.AccountingTimestamps_Name(accounting.LastRollup),
		dbx.AccountingTimestamps_Update_Fields{
			Value: dbx.AccountingTimestamps_Value(latestRollup),
		},
	)
	return Error.Wrap(err)
}

// LastTimestamp records the greatest last tallied time.
func (db *StoragenodeAccounting) LastTimestamp(ctx context.Context, timestampType string) (_ time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	lastTally := time.Time{}
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		lt, err := tx.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(timestampType))
		if lt == nil {
			return tx.ReplaceNoReturn_AccountingTimestamps(ctx,
				dbx.AccountingTimestamps_Name(timestampType),
				dbx.AccountingTimestamps_Value(lastTally),
			)
		}
		lastTally = lt.Value
		return err
	})
	return lastTally, err
}

// QueryPaymentInfo queries Overlay, Accounting Rollup on nodeID.
func (db *StoragenodeAccounting) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) (_ []*accounting.CSVRow, err error) {
	defer mon.Task()(&ctx)(&err)
	sqlStmt := `SELECT n.id, n.created_at, r.at_rest_total, r.get_repair_total,
		r.put_repair_total, r.get_audit_total, r.put_total, r.get_total, n.wallet, n.disqualified
		FROM (
			SELECT node_id, SUM(at_rest_total::decimal) AS at_rest_total, SUM(get_repair_total) AS get_repair_total,
			SUM(put_repair_total) AS put_repair_total, SUM(get_audit_total) AS get_audit_total,
			SUM(put_total) AS put_total, SUM(get_total) AS get_total
			FROM accounting_rollups
			WHERE start_time >= ? AND start_time < ?
			GROUP BY node_id
		) r
		LEFT JOIN nodes n ON n.id = r.node_id
		ORDER BY n.id`

	rows, err := db.db.DB.QueryContext(ctx, db.db.Rebind(sqlStmt), start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	csv := []*accounting.CSVRow{}
	for rows.Next() {
		var nodeID []byte
		r := &accounting.CSVRow{}
		var wallet sql.NullString
		var disqualified *time.Time
		err := rows.Scan(&nodeID, &r.NodeCreationDate, &r.AtRestTotal, &r.GetRepairTotal,
			&r.PutRepairTotal, &r.GetAuditTotal, &r.PutTotal, &r.GetTotal, &wallet, &disqualified)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		if wallet.Valid {
			r.Wallet = wallet.String
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		r.NodeID = id
		r.Disqualified = disqualified
		csv = append(csv, r)
	}
	return csv, rows.Err()
}

// QueryStorageNodePeriodUsage returns usage invoices for nodes for a compensation period.
func (db *StoragenodeAccounting) QueryStorageNodePeriodUsage(ctx context.Context, period compensation.Period) (_ []accounting.StorageNodePeriodUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := db.db.Rebind(`
		SELECT
			node_id,
			SUM(at_rest_total::decimal) AS at_rest_total,
			SUM(get_total) AS get_total,
			SUM(put_total) AS put_total,
			SUM(get_repair_total) AS get_repair_total,
			SUM(put_repair_total) AS put_repair_total,
			SUM(get_audit_total) AS get_audit_total
		FROM
			accounting_rollups
		WHERE
			start_time >= ? AND start_time < ?
		GROUP BY
			node_id
		ORDER BY
			node_id ASC
	`)

	rows, err := db.db.DB.QueryContext(ctx, stmt, period.StartDate(), period.EndDateExclusive())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	usages := []accounting.StorageNodePeriodUsage{}
	for rows.Next() {
		var nodeID []byte
		usage := accounting.StorageNodePeriodUsage{}
		if err := rows.Scan(
			&nodeID,
			&usage.AtRestTotal,
			&usage.GetTotal,
			&usage.PutTotal,
			&usage.GetRepairTotal,
			&usage.PutRepairTotal,
			&usage.GetAuditTotal,
		); err != nil {
			return nil, Error.Wrap(err)
		}

		usage.NodeID, err = storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		usages = append(usages, usage)
	}
	return usages, rows.Err()
}

// QueryStorageNodeUsage returns slice of StorageNodeUsage for given period.
func (db *StoragenodeAccounting) QueryStorageNodeUsage(ctx context.Context, nodeID storj.NodeID, start time.Time, end time.Time) (_ []accounting.StorageNodeUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	lastRollup, err := db.db.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastRollup))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if lastRollup == nil {
		return nil, nil
	}

	start, end = start.UTC(), end.UTC()

	// TODO: remove COALESCE when we're sure the interval_end_time in the
	// accounting_rollups table are fully populated or back-filled with
	// the start_time, and the interval_end_time is non-nullable
	query := `
		SELECT SUM(r1.at_rest_total) as at_rest_total,
				(r1.start_time at time zone 'UTC')::date as start_time,
				COALESCE(MAX(r1.interval_end_time), MAX(r1.start_time)) AS interval_end_time
		FROM accounting_rollups r1
		WHERE r1.node_id = $1
		AND $2 <= r1.start_time AND r1.start_time <= $3
		GROUP BY (r1.start_time at time zone 'UTC')::date
		UNION
		SELECT SUM(t.data_total) AS at_rest_total, (t.interval_end_time at time zone 'UTC')::date AS start_time,
				MAX(t.interval_end_time) AS interval_end_time
				FROM storagenode_storage_tallies t
				WHERE t.node_id = $1
				AND NOT EXISTS (
					SELECT 1 FROM accounting_rollups r2
					WHERE r2.node_id = $1
					AND $2 <= r2.start_time AND r2.start_time <= $3
					AND (r2.start_time at time zone 'UTC')::date = (t.interval_end_time at time zone 'UTC')::date
				)
				AND (SELECT value FROM accounting_timestamps WHERE name = $4) < t.interval_end_time AND t.interval_end_time <= $3
				GROUP BY (t.interval_end_time at time zone 'UTC')::date
		ORDER BY start_time;
	`

	rows, err := db.db.QueryContext(ctx, db.db.Rebind(query),
		nodeID, start, end, accounting.LastRollup,
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var nodeStorageUsages []accounting.StorageNodeUsage
	for rows.Next() {
		var atRestTotal float64
		var startTime, intervalEndTime dbutil.NullTime

		err = rows.Scan(&atRestTotal, &startTime, &intervalEndTime)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		nodeStorageUsages = append(nodeStorageUsages, accounting.StorageNodeUsage{
			NodeID:          nodeID,
			StorageUsed:     atRestTotal,
			Timestamp:       startTime.Time,
			IntervalEndTime: intervalEndTime.Time,
		})
	}

	return nodeStorageUsages, rows.Err()
}

// DeleteTalliesBefore deletes all raw tallies prior to some time.
func (db *StoragenodeAccounting) DeleteTalliesBefore(ctx context.Context, latestRollup time.Time, batchSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if batchSize <= 0 {
		batchSize = 10000
	}

	var query string
	switch db.db.impl {
	case dbutil.Cockroach:
		query = `
			DELETE FROM storagenode_storage_tallies
			WHERE interval_end_time < ?
			LIMIT ?`
	case dbutil.Postgres:
		query = `
			DELETE FROM storagenode_storage_tallies
			WHERE ctid IN (
				SELECT ctid
				FROM storagenode_storage_tallies
				WHERE interval_end_time < ?
				ORDER BY interval_end_time
				LIMIT ?
			)`
	default:
		return Error.New("unsupported database: %v", db.db.impl)
	}
	query = db.db.Rebind(query)

	for {
		res, err := db.db.DB.ExecContext(ctx, query, latestRollup, batchSize)
		if err != nil {
			if errs.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}
		if affected == 0 {
			return nil
		}
	}
}

// ArchiveRollupsBefore archives rollups older than a given time.
func (db *StoragenodeAccounting) ArchiveRollupsBefore(ctx context.Context, before time.Time, batchSize int) (nodeRollupsDeleted int, err error) {
	defer mon.Task()(&ctx)(&err)

	if batchSize <= 0 {
		return 0, nil
	}

	switch db.db.impl {
	case dbutil.Cockroach:
		for {
			row := db.db.QueryRow(ctx, `
			WITH rollups_to_move AS (
				DELETE FROM storagenode_bandwidth_rollups
				WHERE interval_start <= $1
				LIMIT $2 RETURNING *
			), moved_rollups AS (
				INSERT INTO storagenode_bandwidth_rollup_archives SELECT * FROM rollups_to_move RETURNING *
			)
			SELECT count(*) FROM moved_rollups
			`, before, batchSize)

			var rowCount int
			err = row.Scan(&rowCount)
			if err != nil {
				return nodeRollupsDeleted, err
			}
			nodeRollupsDeleted += rowCount

			if rowCount < batchSize {
				break
			}
		}
		return nodeRollupsDeleted, nil

	case dbutil.Postgres:
		storagenodeStatement := `
			WITH rollups_to_move AS (
				DELETE FROM storagenode_bandwidth_rollups
				WHERE interval_start <= $1
				RETURNING *
			), moved_rollups AS (
				INSERT INTO storagenode_bandwidth_rollup_archives SELECT * FROM rollups_to_move RETURNING *
			)
			SELECT count(*) FROM moved_rollups
		`
		row := db.db.DB.QueryRow(ctx, storagenodeStatement, before)
		err = row.Scan(&nodeRollupsDeleted)
		return nodeRollupsDeleted, err

	default:
		return 0, Error.New("unsupported database: %v", db.db.impl)
	}
}

// GetRollupsSince retrieves all archived bandwidth rollup records since a given time.
func (db *StoragenodeAccounting) GetRollupsSince(ctx context.Context, since time.Time) (bwRollups []accounting.StoragenodeBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_StoragenodeBandwidthRollup_By_IntervalStart_GreaterOrEqual_Continuation
	for {
		dbxRollups, next, err := db.db.Paged_StoragenodeBandwidthRollup_By_IntervalStart_GreaterOrEqual(ctx,
			dbx.StoragenodeBandwidthRollup_IntervalStart(since),
			pageLimit, cursor)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		cursor = next

		rollups, err := slices2.Convert(dbxRollups, fromDBXStoragenodeBandwidthRollup)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		bwRollups = append(bwRollups, rollups...)

		if cursor == nil {
			return bwRollups, nil
		}
	}
}

// GetArchivedRollupsSince retrieves all archived bandwidth rollup records since a given time.
func (db *StoragenodeAccounting) GetArchivedRollupsSince(ctx context.Context, since time.Time) (bwRollups []accounting.StoragenodeBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_StoragenodeBandwidthRollupArchive_By_IntervalStart_GreaterOrEqual_Continuation
	for {
		dbxRollups, next, err := db.db.Paged_StoragenodeBandwidthRollupArchive_By_IntervalStart_GreaterOrEqual(ctx,
			dbx.StoragenodeBandwidthRollupArchive_IntervalStart(since),
			pageLimit, cursor)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		cursor = next

		rollups, err := slices2.Convert(dbxRollups, fromDBXStoragenodeBandwidthRollupArchive)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		bwRollups = append(bwRollups, rollups...)

		if cursor == nil {
			return bwRollups, nil
		}
	}
}

func fromDBXStoragenodeStorageTally(r *dbx.StoragenodeStorageTally) (*accounting.StoragenodeStorageTally, error) {
	nodeID, err := storj.NodeIDFromBytes(r.NodeId)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &accounting.StoragenodeStorageTally{
		NodeID:          nodeID,
		IntervalEndTime: r.IntervalEndTime,
		DataTotal:       r.DataTotal,
	}, nil
}

func fromDBXStoragenodeBandwidthRollup(v *dbx.StoragenodeBandwidthRollup) (r accounting.StoragenodeBandwidthRollup, _ error) {
	id, err := storj.NodeIDFromBytes(v.StoragenodeId)
	if err != nil {
		return r, Error.Wrap(err)
	}
	return accounting.StoragenodeBandwidthRollup{
		NodeID:        id,
		IntervalStart: v.IntervalStart,
		Action:        v.Action,
		Settled:       v.Settled,
	}, nil
}

func fromDBXStoragenodeBandwidthRollupPhase2(v *dbx.StoragenodeBandwidthRollupPhase2) (r accounting.StoragenodeBandwidthRollup, _ error) {
	id, err := storj.NodeIDFromBytes(v.StoragenodeId)
	if err != nil {
		return r, Error.Wrap(err)
	}
	return accounting.StoragenodeBandwidthRollup{
		NodeID:        id,
		IntervalStart: v.IntervalStart,
		Action:        v.Action,
		Settled:       v.Settled,
	}, nil
}

func fromDBXStoragenodeBandwidthRollupArchive(v *dbx.StoragenodeBandwidthRollupArchive) (r accounting.StoragenodeBandwidthRollup, _ error) {
	id, err := storj.NodeIDFromBytes(v.StoragenodeId)
	if err != nil {
		return r, Error.Wrap(err)
	}
	return accounting.StoragenodeBandwidthRollup{
		NodeID:        id,
		IntervalStart: v.IntervalStart,
		Action:        v.Action,
		Settled:       v.Settled,
	}, nil
}
