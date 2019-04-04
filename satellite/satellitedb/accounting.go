// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//database implements DB
type accountingDB struct {
	db *dbx.DB
}

// ProjectBandwidthTotal returns the sum of GET bandwidth usage for a projectID for a time frame
func (db *accountingDB) ProjectBandwidthTotal(ctx context.Context, bucketID []byte, from time.Time) (int64, error) {
	pathEl := bytes.Split(bucketID, []byte("/"))
	_, projectID := pathEl[1], pathEl[0]
	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE project_id = ? AND action = ? AND interval_start > ?;`
	err := db.db.QueryRow(db.db.Rebind(query), projectID, pb.PieceAction_GET, from).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}

	return *sum, err
}

// ProjectStorageTotals returns the current inline and remote storage usage for a projectID
func (db *accountingDB) ProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (int64, int64, error) {
	rollup, err := db.db.First_BucketStorageTally_By_ProjectId_OrderBy_Desc_IntervalStart(
		ctx,
		dbx.BucketStorageTally_ProjectId(projectID[:]),
	)
	if err != nil || rollup == nil {
		return 0, 0, err
	}
	return int64(rollup.Inline), int64(rollup.Remote), err
}

// CreateBucketStorageTally creates a record in the bucket_storage_tallies accounting table
func (db *accountingDB) CreateBucketStorageTally(ctx context.Context, tally accounting.BucketStorageTally) error {
	_, err := db.db.Create_BucketStorageTally(
		ctx,
		dbx.BucketStorageTally_BucketName([]byte(tally.BucketName)),
		dbx.BucketStorageTally_ProjectId(tally.ProjectID[:]),
		dbx.BucketStorageTally_IntervalStart(tally.IntervalStart),
		dbx.BucketStorageTally_Inline(uint64(tally.InlineBytes)),
		dbx.BucketStorageTally_Remote(uint64(tally.RemoteBytes)),
		dbx.BucketStorageTally_RemoteSegmentsCount(uint(tally.RemoteSegmentCount)),
		dbx.BucketStorageTally_InlineSegmentsCount(uint(tally.InlineSegmentCount)),
		dbx.BucketStorageTally_ObjectCount(uint(tally.ObjectCount)),
		dbx.BucketStorageTally_MetadataSize(uint64(tally.MetadataSize)),
	)
	if err != nil {
		return err
	}
	return nil
}

// LastTimestamp records the greatest last tallied time
func (db *accountingDB) LastTimestamp(ctx context.Context, timestampType string) (time.Time, error) {
	lastTally := time.Time{}
	err := db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		lt, err := tx.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(timestampType))
		if lt == nil {
			update := dbx.AccountingTimestamps_Value(lastTally)
			_, err = tx.Create_AccountingTimestamps(ctx, dbx.AccountingTimestamps_Name(timestampType), update)
			return err
		}
		lastTally = lt.Value
		return err
	})
	return lastTally, err
}

// SaveAtRestRaw records raw tallies of at rest data to the database
func (db *accountingDB) SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error {
	if len(nodeData) == 0 {
		return Error.New("In SaveAtRestRaw with empty nodeData")
	}
	err := db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for k, v := range nodeData {
			nID := dbx.AccountingRaw_NodeId(k.Bytes())
			end := dbx.AccountingRaw_IntervalEndTime(latestTally)
			total := dbx.AccountingRaw_DataTotal(v)
			dataType := dbx.AccountingRaw_DataType(accounting.AtRest)
			timestamp := dbx.AccountingRaw_CreatedAt(created)
			_, err := tx.Create_AccountingRaw(ctx, nID, end, total, dataType, timestamp)
			if err != nil {
				return err
			}
		}
		update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestTally)}
		_, err := tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastAtRestTally), update)
		return err
	})
	return Error.Wrap(err)
}

// GetRaw retrieves all raw tallies
func (db *accountingDB) GetRaw(ctx context.Context) ([]*accounting.Raw, error) {
	raws, err := db.db.All_AccountingRaw(ctx)
	out := make([]*accounting.Raw, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.Raw{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
			DataType:        r.DataType,
			CreatedAt:       r.CreatedAt,
		}
	}
	return out, Error.Wrap(err)
}

// GetRawSince retrieves all raw tallies since latestRollup
func (db *accountingDB) GetRawSince(ctx context.Context, latestRollup time.Time) ([]*accounting.Raw, error) {
	raws, err := db.db.All_AccountingRaw_By_IntervalEndTime_GreaterOrEqual(ctx, dbx.AccountingRaw_IntervalEndTime(latestRollup))
	out := make([]*accounting.Raw, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.Raw{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
			DataType:        r.DataType,
			CreatedAt:       r.CreatedAt,
		}
	}
	return out, Error.Wrap(err)
}

// GetStoragenodeBandwidthSince retrieves all storagenode_bandwidth_rollup entires since latestRollup
func (db *accountingDB) GetStoragenodeBandwidthSince(ctx context.Context, latestRollup time.Time) ([]*accounting.StoragenodeBandwidthRollup, error) {
	rollups, err := db.db.All_StoragenodeBandwidthRollup_By_IntervalStart_GreaterOrEqual(ctx, dbx.StoragenodeBandwidthRollup_IntervalStart(latestRollup))
	out := make([]*accounting.StoragenodeBandwidthRollup, len(rollups))
	for i, r := range rollups {
		nodeID, err := storj.NodeIDFromBytes(r.StoragenodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.StoragenodeBandwidthRollup{
			NodeID:        nodeID,
			IntervalStart: r.IntervalStart,
			Action:        r.Action,
			Settled:       r.Settled,
		}
	}
	return out, Error.Wrap(err)
}

// SaveRollup records raw tallies of at rest data to the database
func (db *accountingDB) SaveRollup(ctx context.Context, latestRollup time.Time, stats accounting.RollupStats) error {
	if len(stats) == 0 {
		return Error.New("In SaveRollup with empty nodeData")
	}
	err := db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, arsByDate := range stats {
			for _, ar := range arsByDate {
				nID := dbx.AccountingRollup_NodeId(ar.NodeID.Bytes())
				start := dbx.AccountingRollup_StartTime(ar.StartTime)
				put := dbx.AccountingRollup_PutTotal(ar.PutTotal)
				get := dbx.AccountingRollup_GetTotal(ar.GetTotal)
				audit := dbx.AccountingRollup_GetAuditTotal(ar.GetAuditTotal)
				getRepair := dbx.AccountingRollup_GetRepairTotal(ar.GetRepairTotal)
				putRepair := dbx.AccountingRollup_PutRepairTotal(ar.PutRepairTotal)
				atRest := dbx.AccountingRollup_AtRestTotal(ar.AtRestTotal)
				_, err := tx.Create_AccountingRollup(ctx, nID, start, put, get, audit, getRepair, putRepair, atRest)
				if err != nil {
					return err
				}
			}
		}
		update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestRollup)}
		_, err := tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastRollup), update)
		return err
	})
	return Error.Wrap(err)
}

// SaveBucketTallies saves the latest bucket info
func (db *accountingDB) SaveBucketTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*accounting.BucketTally) error {
	if len(bucketTallies) == 0 {
		return Error.New("In SaveBucketTallies with empty bucketTallies")
	}

	for bucketID, info := range bucketTallies {
		bucketIDComponents := storj.SplitPath(bucketID)
		bucketName := dbx.BucketStorageTally_BucketName([]byte(bucketIDComponents[1]))
		projectID := dbx.BucketStorageTally_ProjectId([]byte(bucketIDComponents[0]))
		interval := dbx.BucketStorageTally_IntervalStart(intervalStart)
		inlineBytes := dbx.BucketStorageTally_Inline(uint64(info.InlineBytes))
		remoteBytes := dbx.BucketStorageTally_Remote(uint64(info.RemoteBytes))
		rSegments := dbx.BucketStorageTally_RemoteSegmentsCount(uint(info.RemoteSegments))
		iSegments := dbx.BucketStorageTally_InlineSegmentsCount(uint(info.InlineSegments))
		objectCount := dbx.BucketStorageTally_ObjectCount(uint(info.Files))
		meta := dbx.BucketStorageTally_MetadataSize(uint64(info.MetadataSize))
		_, err := db.db.Create_BucketStorageTally(ctx, bucketName, projectID, interval, inlineBytes, remoteBytes, rSegments, iSegments, objectCount, meta)
		if err != nil {
			return err
		}
	}
	return nil
}

// QueryPaymentInfo queries Overlay, Accounting Rollup on nodeID
func (db *accountingDB) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*accounting.CSVRow, error) {
	var sqlStmt = `SELECT n.id, n.created_at, n.audit_success_ratio, r.at_rest_total, r.get_repair_total,
	    r.put_repair_total, r.get_audit_total, r.put_total, r.get_total, n.wallet
	    FROM (
			SELECT node_id, SUM(at_rest_total) AS at_rest_total, SUM(get_repair_total) AS get_repair_total,
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
	csv := make([]*accounting.CSVRow, 0, 0)
	for rows.Next() {
		var nodeID []byte
		r := &accounting.CSVRow{}
		var wallet sql.NullString
		err := rows.Scan(&nodeID, &r.NodeCreationDate, &r.AuditSuccessRatio, &r.AtRestTotal, &r.GetRepairTotal,
			&r.PutRepairTotal, &r.GetAuditTotal, &r.PutTotal, &r.GetTotal, &wallet)
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
		csv = append(csv, r)
	}
	return csv, nil
}

// DeleteRawBefore deletes all raw tallies prior to some time
func (db *accountingDB) DeleteRawBefore(ctx context.Context, latestRollup time.Time) error {
	var deleteRawSQL = `DELETE FROM accounting_raws WHERE interval_end_time < ?`
	_, err := db.db.DB.ExecContext(ctx, db.db.Rebind(deleteRawSQL), latestRollup)
	return err
}
