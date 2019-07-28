// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ProjectAccounting implements the accounting/db ProjectAccounting interface
type ProjectAccounting struct {
	db *dbx.DB
}

// SaveTallies saves the latest bucket info
func (db *ProjectAccounting) SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*accounting.BucketTally) (_ []accounting.BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(bucketTallies) == 0 {
		return nil, Error.New("In SaveTallies with empty bucketTallies")
	}

	var result []accounting.BucketTally

	for _, info := range bucketTallies {
		bucketName := dbx.BucketStorageTally_BucketName(info.BucketName)
		projectID := dbx.BucketStorageTally_ProjectId(info.ProjectID)
		interval := dbx.BucketStorageTally_IntervalStart(intervalStart)
		inlineBytes := dbx.BucketStorageTally_Inline(uint64(info.InlineBytes))
		remoteBytes := dbx.BucketStorageTally_Remote(uint64(info.RemoteBytes))
		rSegments := dbx.BucketStorageTally_RemoteSegmentsCount(uint(info.RemoteSegments))
		iSegments := dbx.BucketStorageTally_InlineSegmentsCount(uint(info.InlineSegments))
		objectCount := dbx.BucketStorageTally_ObjectCount(uint(info.Files))
		meta := dbx.BucketStorageTally_MetadataSize(uint64(info.MetadataSize))
		dbxTally, err := db.db.Create_BucketStorageTally(ctx, bucketName, projectID, interval, inlineBytes, remoteBytes, rSegments, iSegments, objectCount, meta)
		if err != nil {
			return nil, err
		}
		tally := accounting.BucketTally{
			BucketName:     dbxTally.BucketName,
			ProjectID:      dbxTally.ProjectId,
			InlineSegments: int64(dbxTally.InlineSegmentsCount),
			RemoteSegments: int64(dbxTally.RemoteSegmentsCount),
			Files:          int64(dbxTally.ObjectCount),
			InlineBytes:    int64(dbxTally.Inline),
			RemoteBytes:    int64(dbxTally.Remote),
			MetadataSize:   int64(dbxTally.MetadataSize),
		}
		result = append(result, tally)
	}
	return result, nil
}

// CreateStorageTally creates a record in the bucket_storage_tallies accounting table
func (db *ProjectAccounting) CreateStorageTally(ctx context.Context, tally accounting.BucketStorageTally) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Create_BucketStorageTally(
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

// GetProjectUsageLimits returns project usage limit
func (db *ProjectAccounting) GetProjectUsageLimits(ctx context.Context, projectID uuid.UUID) (_ memory.Size, err error) {
	defer mon.Task()(&ctx)(&err)
	project, err := db.db.Get_Project_By_Id(ctx, dbx.Project_Id(projectID[:]))
	if err != nil {
		return 0, err
	}
	return memory.Size(project.UsageLimit), nil
}
