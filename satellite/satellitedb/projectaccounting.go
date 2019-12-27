// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that ProjectAccounting implements accounting.ProjectAccounting.
var _ accounting.ProjectAccounting = (*ProjectAccounting)(nil)

// ProjectAccounting implements the accounting/db ProjectAccounting interface
type ProjectAccounting struct {
	db *satelliteDB
}

// SaveTallies saves the latest bucket info
func (db *ProjectAccounting) SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*accounting.BucketTally) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(bucketTallies) == 0 {
		return nil
	}

	// TODO: see if we can send all bucket storage tallies to the db in one operation
	return Error.Wrap(db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, info := range bucketTallies {
			err := tx.CreateNoReturn_BucketStorageTally(ctx,
				dbx.BucketStorageTally_BucketName(info.BucketName),
				dbx.BucketStorageTally_ProjectId(info.ProjectID[:]),
				dbx.BucketStorageTally_IntervalStart(intervalStart),
				dbx.BucketStorageTally_Inline(uint64(info.InlineBytes)),
				dbx.BucketStorageTally_Remote(uint64(info.RemoteBytes)),
				dbx.BucketStorageTally_RemoteSegmentsCount(uint(info.RemoteSegments)),
				dbx.BucketStorageTally_InlineSegmentsCount(uint(info.InlineSegments)),
				dbx.BucketStorageTally_ObjectCount(uint(info.ObjectCount)),
				dbx.BucketStorageTally_MetadataSize(uint64(info.MetadataSize)),
			)
			if err != nil {
				return Error.Wrap(err)
			}
		}
		return nil
	}))
}

// GetTallies saves the latest bucket info
func (db *ProjectAccounting) GetTallies(ctx context.Context) (tallies []accounting.BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTallies, err := db.db.All_BucketStorageTally(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, dbxTally := range dbxTallies {
		projectID, err := dbutil.BytesToUUID(dbxTally.ProjectId)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		tallies = append(tallies, accounting.BucketTally{
			BucketName:     dbxTally.BucketName,
			ProjectID:      projectID,
			ObjectCount:    int64(dbxTally.ObjectCount),
			InlineSegments: int64(dbxTally.InlineSegmentsCount),
			RemoteSegments: int64(dbxTally.RemoteSegmentsCount),
			InlineBytes:    int64(dbxTally.Inline),
			RemoteBytes:    int64(dbxTally.Remote),
			MetadataSize:   int64(dbxTally.MetadataSize),
		})
	}

	return tallies, nil
}

// CreateStorageTally creates a record in the bucket_storage_tallies accounting table
func (db *ProjectAccounting) CreateStorageTally(ctx context.Context, tally accounting.BucketStorageTally) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(db.db.CreateNoReturn_BucketStorageTally(
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
	))
}

// GetAllocatedBandwidthTotal returns the sum of GET bandwidth usage allocated for a projectID for a time frame
func (db *ProjectAccounting) GetAllocatedBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(allocated) FROM bucket_bandwidth_rollups WHERE project_id = ? AND action = ? AND interval_start > ?;`
	err = db.db.QueryRow(db.db.Rebind(query), projectID[:], pb.PieceAction_GET, from).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}

	return *sum, err
}

// GetStorageTotals returns the current inline and remote storage usage for a projectID
func (db *ProjectAccounting) GetStorageTotals(ctx context.Context, projectID uuid.UUID) (inline int64, remote int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var inlineSum, remoteSum sql.NullInt64
	var intervalStart time.Time

	// Sum all the inline and remote values for a project that all share the same interval_start.
	// All records for a project that have the same interval start are part of the same tally run.
	// This should represent the most recent calculation of a project's total at rest storage.
	query := `SELECT interval_start, SUM(inline), SUM(remote)
		FROM bucket_storage_tallies
		WHERE project_id = ?
		GROUP BY interval_start
		ORDER BY interval_start DESC LIMIT 1;`

	err = db.db.QueryRow(db.db.Rebind(query), projectID[:]).Scan(&intervalStart, &inlineSum, &remoteSum)
	if err != nil || !inlineSum.Valid || !remoteSum.Valid {
		return 0, 0, nil
	}
	return inlineSum.Int64, remoteSum.Int64, err
}

// UpdateProjectUsageLimit updates project usage limit.
func (db *ProjectAccounting) UpdateProjectUsageLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
		dbx.Project_Update_Fields{
			UsageLimit: dbx.Project_UsageLimit(limit.Int64()),
		},
	)

	return err
}

// GetProjectStorageLimit returns project storage usage limit.
func (db *ProjectAccounting) GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.getProjectUsageLimit(ctx, projectID)
}

// GetProjectBandwidthLimit returns project bandwidth usage limit.
func (db *ProjectAccounting) GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.getProjectUsageLimit(ctx, projectID)
}

// getProjectUsageLimit returns project usage limit.
func (db *ProjectAccounting) getProjectUsageLimit(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_Project_UsageLimit_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
	)
	if err != nil {
		return 0, err
	}

	return memory.Size(row.UsageLimit), nil
}

// GetProjectTotal retrieves project usage for a given period.
func (db *ProjectAccounting) GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (usage *accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since)

	bucketNames, err := db.getBuckets(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	storageQuery := db.db.Rebind(`
		SELECT
			bucket_storage_tallies.interval_start, 
			bucket_storage_tallies.inline,
			bucket_storage_tallies.remote,
			bucket_storage_tallies.object_count
		FROM 
			bucket_storage_tallies 
		WHERE 
			bucket_storage_tallies.project_id = ? AND 
			bucket_storage_tallies.bucket_name = ? AND 
			bucket_storage_tallies.interval_start >= ? AND 
			bucket_storage_tallies.interval_start <= ? 
		ORDER BY bucket_storage_tallies.interval_start DESC
	`)

	bucketsTallies := make(map[string][]*accounting.BucketStorageTally)

	for _, bucket := range bucketNames {
		storageTallies := make([]*accounting.BucketStorageTally, 0)
		storageTalliesRows, err := db.db.QueryContext(ctx, storageQuery, projectID[:], []byte(bucket), since, before)
		if err != nil {
			return nil, err
		}

		// generating tallies for each bucket name.
		for storageTalliesRows.Next() {
			tally := accounting.BucketStorageTally{}

			err = storageTalliesRows.Scan(&tally.IntervalStart, &tally.InlineBytes, &tally.RemoteBytes, &tally.ObjectCount)
			if err != nil {
				return nil, err
			}
			tally.BucketName = bucket
			storageTallies = append(storageTallies, &tally)
		}

		err = storageTalliesRows.Close()
		if err != nil {
			return nil, err
		}

		bucketsTallies[bucket] = storageTallies
	}

	totalEgress, err := db.getTotalEgress(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	usage = new(accounting.ProjectUsage)
	usage.Egress = memory.Size(totalEgress).Int64()

	// sum up storage and objects
	for _, tallies := range bucketsTallies {
		for i := len(tallies) - 1; i > 0; i-- {
			current := (tallies)[i]

			hours := (tallies)[i-1].IntervalStart.Sub(current.IntervalStart).Hours()

			usage.Storage += memory.Size(current.InlineBytes).Float64() * hours
			usage.Storage += memory.Size(current.RemoteBytes).Float64() * hours
			usage.ObjectCount += float64(current.ObjectCount) * hours
		}
	}

	usage.Since = since
	usage.Before = before
	return usage, nil
}

// getTotalEgress returns total egress (settled + inline) of each bucket_bandwidth_rollup
// in selected time period, project id.
// only process PieceAction_GET, PieceAction_GET_AUDIT, PieceAction_GET_REPAIR actions.
func (db *ProjectAccounting) getTotalEgress(ctx context.Context, projectID uuid.UUID, since, before time.Time) (totalEgress int64, err error) {
	totalEgressQuery := db.db.Rebind(fmt.Sprintf(`
		SELECT 
			COALESCE(SUM(settled) + SUM(inline), 0)  
		FROM 
			bucket_bandwidth_rollups 
		WHERE 
			project_id = ? AND 
			interval_start >= ? AND 
			interval_start <= ? AND 
			action IN (%d, %d, %d);
	`, pb.PieceAction_GET, pb.PieceAction_GET_AUDIT, pb.PieceAction_GET_REPAIR))

	totalEgressRow := db.db.QueryRowContext(ctx, totalEgressQuery, projectID[:], since, before)

	err = totalEgressRow.Scan(&totalEgress)

	return totalEgress, err
}

// GetBucketUsageRollups retrieves summed usage rollups for every bucket of particular project for a given period
func (db *ProjectAccounting) GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ []accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since)

	buckets, err := db.getBuckets(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	roullupsQuery := db.db.Rebind(`SELECT SUM(settled), SUM(inline), action
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		GROUP BY action`)

	// TODO: should be optimized
	storageQuery := db.db.All_BucketStorageTally_By_ProjectId_And_BucketName_And_IntervalStart_GreaterOrEqual_And_IntervalStart_LessOrEqual_OrderBy_Desc_IntervalStart

	var bucketUsageRollups []accounting.BucketUsageRollup
	for _, bucket := range buckets {
		bucketRollup := accounting.BucketUsageRollup{
			ProjectID:  projectID,
			BucketName: []byte(bucket),
			Since:      since,
			Before:     before,
		}

		// get bucket_bandwidth_rollups
		rollupsRows, err := db.db.QueryContext(ctx, roullupsQuery, projectID[:], []byte(bucket), since, before)
		if err != nil {
			return nil, err
		}
		defer func() { err = errs.Combine(err, rollupsRows.Close()) }()

		// fill egress
		for rollupsRows.Next() {
			var action pb.PieceAction
			var settled, inline int64

			err = rollupsRows.Scan(&settled, &inline, &action)
			if err != nil {
				return nil, err
			}

			switch action {
			case pb.PieceAction_GET:
				bucketRollup.GetEgress += memory.Size(settled + inline).GB()
			case pb.PieceAction_GET_AUDIT:
				bucketRollup.AuditEgress += memory.Size(settled + inline).GB()
			case pb.PieceAction_GET_REPAIR:
				bucketRollup.RepairEgress += memory.Size(settled + inline).GB()
			default:
				continue
			}
		}

		bucketStorageTallies, err := storageQuery(ctx,
			dbx.BucketStorageTally_ProjectId(projectID[:]),
			dbx.BucketStorageTally_BucketName([]byte(bucket)),
			dbx.BucketStorageTally_IntervalStart(since),
			dbx.BucketStorageTally_IntervalStart(before))

		if err != nil {
			return nil, err
		}

		// fill metadata, objects and stored data
		// hours calculated from previous tallies,
		// so we skip the most recent one
		for i := len(bucketStorageTallies) - 1; i > 0; i-- {
			current := bucketStorageTallies[i]

			hours := bucketStorageTallies[i-1].IntervalStart.Sub(current.IntervalStart).Hours()

			bucketRollup.RemoteStoredData += memory.Size(current.Remote).GB() * hours
			bucketRollup.InlineStoredData += memory.Size(current.Inline).GB() * hours
			bucketRollup.MetadataSize += memory.Size(current.MetadataSize).GB() * hours
			bucketRollup.RemoteSegments += float64(current.RemoteSegmentsCount) * hours
			bucketRollup.InlineSegments += float64(current.InlineSegmentsCount) * hours
			bucketRollup.ObjectCount += float64(current.ObjectCount) * hours
		}

		bucketUsageRollups = append(bucketUsageRollups, bucketRollup)
	}

	return bucketUsageRollups, nil
}

// prefixIncrement returns the lexicographically lowest byte string which is
// greater than origPrefix and does not have origPrefix as a prefix. If no such
// byte string exists (origPrefix is empty, or origPrefix contains only 0xff
// bytes), returns false for ok.
//
// examples: prefixIncrement([]byte("abc"))          -> ([]byte("abd", true)
//           prefixIncrement([]byte("ab\xff\xff"))   -> ([]byte("ac", true)
//           prefixIncrement([]byte(""))             -> (nil, false)
//           prefixIncrement([]byte("\x00"))         -> ([]byte("\x01", true)
//           prefixIncrement([]byte("\xff\xff\xff")) -> (nil, false)
//
func prefixIncrement(origPrefix []byte) (incremented []byte, ok bool) {
	incremented = make([]byte, len(origPrefix))
	copy(incremented, origPrefix)
	i := len(incremented) - 1
	for i >= 0 {
		if incremented[i] != 0xff {
			incremented[i]++
			return incremented[:i+1], true
		}
		i--
	}

	// there is no byte string which is greater than origPrefix and which does
	// not have origPrefix as a prefix.
	return nil, false
}

// prefixMatch creates a SQL expression which
// will evaluate to true if and only if the value of expr starts with the value
// of prefix.
//
// Returns also a slice of arguments that should be passed to the corresponding
// db.Query* or db.Exec* to fill in parameters in the returned SQL expression.
//
// The returned SQL expression needs to be passed through Rebind(), as it uses
// `?` markers instead of `$N`, because we don't know what N we would need to
// use.
func (db *ProjectAccounting) prefixMatch(expr string, prefix []byte) (string, []byte, error) {
	incrementedPrefix, ok := prefixIncrement(prefix)
	switch db.db.implementation {
	case dbutil.Postgres:
		if !ok {
			return fmt.Sprintf(`(%s >= ?)`, expr), nil, nil
		}
		return fmt.Sprintf(`(%s >= ? AND %s < ?)`, expr, expr), incrementedPrefix, nil
	case dbutil.Cockroach:
		if !ok {
			return fmt.Sprintf(`(%s >= ?:::BYTEA)`, expr), nil, nil
		}
		return fmt.Sprintf(`(%s >= ?:::BYTEA AND %s < ?:::BYTEA)`, expr, expr), incrementedPrefix, nil
	default:
		return "", nil, errs.New("invalid dbType: %v", db.db.driver)
	}

}

// GetBucketTotals retrieves bucket usage totals for period of time
func (db *ProjectAccounting) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, since, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since)
	bucketPrefix := []byte(cursor.Search)

	if cursor.Limit > 50 {
		cursor.Limit = 50
	}
	if cursor.Page == 0 {
		return nil, errs.New("page can not be 0")
	}

	page := &accounting.BucketUsagePage{
		Search: cursor.Search,
		Limit:  cursor.Limit,
		Offset: uint64((cursor.Page - 1) * cursor.Limit),
	}

	bucketNameRange, incrPrefix, err := db.prefixMatch("bucket_name", bucketPrefix)
	if err != nil {
		return nil, err
	}
	countQuery := db.db.Rebind(`SELECT COUNT(DISTINCT bucket_name)
	FROM bucket_bandwidth_rollups
	WHERE project_id = ? AND interval_start >= ? AND interval_start <= ?
	AND ` + bucketNameRange)

	args := []interface{}{
		projectID[:],
		since,
		before,
		bucketPrefix,
	}
	if incrPrefix != nil {
		args = append(args, incrPrefix)
	}

	countRow := db.db.QueryRowContext(ctx, countQuery, args...)

	err = countRow.Scan(&page.TotalCount)
	if err != nil {
		return nil, err
	}

	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return nil, errs.New("page is out of range")
	}

	var buckets []string
	bucketsQuery := db.db.Rebind(`SELECT DISTINCT bucket_name
	FROM bucket_bandwidth_rollups
	WHERE project_id = ? AND interval_start >= ? AND interval_start <= ?
	AND ` + bucketNameRange + ` ORDER BY bucket_name ASC
	LIMIT ? OFFSET ?`)

	args = []interface{}{
		projectID[:],
		since,
		before,
		bucketPrefix,
	}
	if incrPrefix != nil {
		args = append(args, incrPrefix)
	}
	args = append(args, page.Limit, page.Offset)

	bucketRows, err := db.db.QueryContext(ctx, bucketsQuery, args...)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, bucketRows.Close()) }()
	for bucketRows.Next() {
		var bucket string
		err = bucketRows.Scan(&bucket)
		if err != nil {
			return nil, err
		}

		buckets = append(buckets, bucket)
	}

	rollupsQuery := db.db.Rebind(`SELECT SUM(settled), SUM(inline), action
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		GROUP BY action`)

	storageQuery := db.db.Rebind(`SELECT inline, remote, object_count
		FROM bucket_storage_tallies
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		ORDER BY interval_start DESC
		LIMIT 1`)

	var bucketUsages []accounting.BucketUsage
	for _, bucket := range buckets {
		bucketUsage := accounting.BucketUsage{
			ProjectID:  projectID,
			BucketName: bucket,
			Since:      since,
			Before:     before,
		}

		// get bucket_bandwidth_rollups
		rollupsRows, err := db.db.QueryContext(ctx, rollupsQuery, projectID[:], []byte(bucket), since, before)
		if err != nil {
			return nil, err
		}
		defer func() { err = errs.Combine(err, rollupsRows.Close()) }()

		var totalEgress int64
		for rollupsRows.Next() {
			var action pb.PieceAction
			var settled, inline int64

			err = rollupsRows.Scan(&settled, &inline, &action)
			if err != nil {
				return nil, err
			}

			// add values for egress
			if action == pb.PieceAction_GET || action == pb.PieceAction_GET_AUDIT || action == pb.PieceAction_GET_REPAIR {
				totalEgress += settled + inline
			}
		}

		bucketUsage.Egress = memory.Size(totalEgress).GB()

		storageRow := db.db.QueryRowContext(ctx, storageQuery, projectID[:], []byte(bucket), since, before)
		if err != nil {
			return nil, err
		}

		var inline, remote, objectCount int64
		err = storageRow.Scan(&inline, &remote, &objectCount)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, err
			}
		}

		// fill storage and object count
		bucketUsage.Storage = memory.Size(inline + remote).GB()
		bucketUsage.ObjectCount = objectCount

		bucketUsages = append(bucketUsages, bucketUsage)
	}

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.BucketUsages = bucketUsages
	page.CurrentPage = cursor.Page
	return page, nil
}

// getBuckets list all bucket of certain project for given period
func (db *ProjectAccounting) getBuckets(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)
	bucketsQuery := db.db.Rebind(`SELECT DISTINCT bucket_name
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND interval_start >= ? AND interval_start <= ?`)

	bucketRows, err := db.db.QueryContext(ctx, bucketsQuery, projectID[:], since, before)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, bucketRows.Close()) }()

	var buckets []string
	for bucketRows.Next() {
		var bucket string
		err = bucketRows.Scan(&bucket)
		if err != nil {
			return nil, err
		}

		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

// timeTruncateDown truncates down to the hour before to be in sync with orders endpoint
func timeTruncateDown(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
