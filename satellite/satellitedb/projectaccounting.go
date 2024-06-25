// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	satbuckets "storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/tagsql"
)

// ensure that ProjectAccounting implements accounting.ProjectAccounting.
var _ accounting.ProjectAccounting = (*ProjectAccounting)(nil)

// maxLimit specifies the limit for all paged queries.
const maxLimit = 300

var allocatedExpirationInDays = 2

// ProjectAccounting implements the accounting/db ProjectAccounting interface.
type ProjectAccounting struct {
	db *satelliteDB
}

// SaveTallies saves the latest bucket info.
func (db *ProjectAccounting) SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[metabase.BucketLocation]*accounting.BucketTally) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(bucketTallies) == 0 {
		return nil
	}
	var bucketNames, projectIDs [][]byte
	var totalBytes, metadataSizes []int64
	var totalSegments, objectCounts []int64
	for _, info := range bucketTallies {
		bucketNames = append(bucketNames, []byte(info.BucketName))
		projectIDs = append(projectIDs, info.ProjectID[:])
		totalBytes = append(totalBytes, info.TotalBytes)
		totalSegments = append(totalSegments, info.TotalSegments)
		objectCounts = append(objectCounts, info.ObjectCount)
		metadataSizes = append(metadataSizes, info.MetadataSize)
	}
	_, err = db.db.DB.ExecContext(ctx, db.db.Rebind(`
		INSERT INTO bucket_storage_tallies (
			interval_start,
			bucket_name, project_id,
			total_bytes, inline, remote,
			total_segments_count, remote_segments_count, inline_segments_count,
			object_count, metadata_size)
		SELECT
			$1,
			unnest($2::bytea[]), unnest($3::bytea[]),
			unnest($4::int8[]), $5, $6,
			unnest($7::int8[]), $8, $9,
			unnest($10::int8[]), unnest($11::int8[])`),
		intervalStart,
		pgutil.ByteaArray(bucketNames), pgutil.ByteaArray(projectIDs),
		pgutil.Int8Array(totalBytes), 0, 0,
		pgutil.Int8Array(totalSegments), 0, 0,
		pgutil.Int8Array(objectCounts), pgutil.Int8Array(metadataSizes))

	return Error.Wrap(err)
}

// GetTallies retrieves all tallies ordered by interval start (descending).
func (db *ProjectAccounting) GetTallies(ctx context.Context) (tallies []accounting.BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTallies, err := db.db.All_BucketStorageTally_OrderBy_Desc_IntervalStart(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, dbxTally := range dbxTallies {
		projectID, err := uuid.FromBytes(dbxTally.ProjectId)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		totalBytes := dbxTally.TotalBytes
		if totalBytes == 0 {
			totalBytes = dbxTally.Inline + dbxTally.Remote
		}

		totalSegments := dbxTally.TotalSegmentsCount
		if totalSegments == 0 {
			totalSegments = dbxTally.InlineSegmentsCount + dbxTally.RemoteSegmentsCount
		}

		tallies = append(tallies, accounting.BucketTally{
			BucketLocation: metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: string(dbxTally.BucketName),
			},
			ObjectCount:   int64(dbxTally.ObjectCount),
			TotalSegments: int64(totalSegments),
			TotalBytes:    int64(totalBytes),
			MetadataSize:  int64(dbxTally.MetadataSize),
		})
	}

	return tallies, nil
}

// CreateStorageTally creates a record in the bucket_storage_tallies accounting table.
func (db *ProjectAccounting) CreateStorageTally(ctx context.Context, tally accounting.BucketStorageTally) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.DB.ExecContext(ctx, db.db.Rebind(`
		INSERT INTO bucket_storage_tallies (
			interval_start,
			bucket_name, project_id,
			total_bytes, inline, remote,
			total_segments_count, remote_segments_count, inline_segments_count,
			object_count, metadata_size)
		VALUES (
			?,
			?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?
		)`), tally.IntervalStart,
		[]byte(tally.BucketName), tally.ProjectID,
		tally.TotalBytes, 0, 0,
		tally.TotalSegmentCount, 0, 0,
		tally.ObjectCount, tally.MetadataSize,
	)

	return Error.Wrap(err)
}

// GetNonEmptyTallyBucketsInRange returns a list of bucket locations within the given range
// whose most recent tally does not represent empty usage.
func (db *ProjectAccounting) GetNonEmptyTallyBucketsInRange(ctx context.Context, from, to metabase.BucketLocation, asOfSystemInterval time.Duration) (result []metabase.BucketLocation, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(db.db.QueryContext(ctx, `
		SELECT project_id, name
		FROM bucket_metainfos bm`+
		db.db.impl.AsOfSystemInterval(asOfSystemInterval)+
		` WHERE (project_id, name) BETWEEN ($1, $2) AND ($3, $4)
		AND NOT 0 IN (
			SELECT object_count FROM bucket_storage_tallies
			WHERE (project_id, bucket_name) = (bm.project_id, bm.name)
			ORDER BY interval_start DESC
			LIMIT 1
		)
	`, from.ProjectID, []byte(from.BucketName), to.ProjectID, []byte(to.BucketName)),
	)(func(r tagsql.Rows) error {
		for r.Next() {
			loc := metabase.BucketLocation{}
			if err := r.Scan(&loc.ProjectID, &loc.BucketName); err != nil {
				return err
			}
			result = append(result, loc)
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// GetProjectSettledBandwidthTotal returns the sum of GET bandwidth usage settled for a projectID in the past time frame.
func (db *ProjectAccounting) GetProjectSettledBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE project_id = $1 AND action = $2 AND interval_start >= $3;`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), projectID[:], pb.PieceAction_GET, from.UTC()).Scan(&sum)
	if errors.Is(err, sql.ErrNoRows) || sum == nil {
		return 0, nil
	}

	return *sum, err
}

// GetProjectBandwidth returns the used bandwidth (settled or allocated) for the specified year, month and day.
func (db *ProjectAccounting) GetProjectBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int, asOfSystemInterval time.Duration) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var egress *int64

	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	var expiredSince time.Time
	if day < allocatedExpirationInDays {
		expiredSince = startOfMonth
	} else {
		expiredSince = time.Date(year, month, day-allocatedExpirationInDays, 0, 0, 0, 0, time.UTC)
	}
	periodEnd := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)

	query := `WITH egress AS (
					SELECT
						CASE WHEN interval_day < ?
							THEN egress_settled
							ELSE egress_allocated-egress_dead
						END AS amount
					FROM project_bandwidth_daily_rollups
					WHERE project_id = ? AND interval_day >= ? AND interval_day < ?
				) SELECT sum(amount) FROM egress` + db.db.impl.AsOfSystemInterval(asOfSystemInterval)
	err = db.db.QueryRow(ctx, db.db.Rebind(query), expiredSince, projectID[:], startOfMonth, periodEnd).Scan(&egress)
	if errors.Is(err, sql.ErrNoRows) || egress == nil {
		return 0, nil
	}

	return *egress, err
}

// GetProjectSettledBandwidth returns the used settled bandwidth for the specified year and month.
func (db *ProjectAccounting) GetProjectSettledBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, asOfSystemInterval time.Duration) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var egress *int64

	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)

	query := `SELECT sum(egress_settled) FROM project_bandwidth_daily_rollups` +
		db.db.impl.AsOfSystemInterval(asOfSystemInterval) +
		` WHERE project_id = ? AND interval_day >= ? AND interval_day < ?`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), projectID[:], startOfMonth, periodEnd).Scan(&egress)
	if errors.Is(err, sql.ErrNoRows) || egress == nil {
		return 0, nil
	}

	return *egress, err
}

// GetProjectDailyBandwidth returns project bandwidth (allocated and settled) for the specified day.
func (db *ProjectAccounting) GetProjectDailyBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int) (allocated int64, settled, dead int64, err error) {
	defer mon.Task()(&ctx)(&err)

	interval := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	query := `SELECT egress_allocated, egress_settled, egress_dead FROM project_bandwidth_daily_rollups WHERE project_id = ? AND interval_day = ?;`
	err = db.db.QueryRow(ctx, db.db.Rebind(query), projectID[:], interval).Scan(&allocated, &settled, &dead)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, nil
	}

	return allocated, settled, dead, err
}

// GetProjectDailyUsageByDateRange returns project daily allocated, settled bandwidth and storage usage by specific date range.
func (db *ProjectAccounting) GetProjectDailyUsageByDateRange(ctx context.Context, projectID uuid.UUID, from, to time.Time, crdbInterval time.Duration) (_ *accounting.ProjectDailyUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()
	nowBeginningOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	fromBeginningOfDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toEndOfDay := time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, time.UTC)

	allocatedBandwidth := make([]accounting.ProjectUsageByDay, 0)
	settledBandwidth := make([]accounting.ProjectUsageByDay, 0)
	storage := make([]accounting.ProjectUsageByDay, 0)

	err = pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch

		expiredSince := nowBeginningOfDay.Add(time.Duration(-allocatedExpirationInDays) * time.Hour * 24)

		storageQuery := db.db.Rebind(`
			WITH project_usage AS (
			SELECT
				interval_start,
				DATE_TRUNC('day',interval_start) AS interval_day,
				project_id,
				bucket_name,
				total_bytes
			FROM bucket_storage_tallies
			WHERE project_id = $1 AND
				interval_start >= $2 AND
				interval_start <= $3
			)
			-- Sum all buckets usage in the same project.
			SELECT
				interval_day,
				SUM(total_bytes) AS total_bytes
			FROM
				(SELECT
					DISTINCT ON (project_id, bucket_name, interval_day)
					project_id,
					bucket_name,
					total_bytes,
					interval_day,
					interval_start
				FROM project_usage
				ORDER BY project_id, bucket_name, interval_day, interval_start DESC) pu
			` + db.db.impl.AsOfSystemInterval(crdbInterval) + `
			GROUP BY project_id, interval_day
		`)
		batch.Queue(storageQuery, projectID, fromBeginningOfDay, toEndOfDay)

		batch.Queue(db.db.Rebind(`
			SELECT interval_day, egress_settled,
				CASE WHEN interval_day < $1
					THEN egress_settled
					ELSE egress_allocated-egress_dead
				END AS allocated
			FROM project_bandwidth_daily_rollups
			WHERE project_id = $2 AND (interval_day BETWEEN $3 AND $4)
		`), expiredSince, projectID, fromBeginningOfDay, toEndOfDay)

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		storageRows, err := results.Query()
		if err != nil {
			if pgerrcode.FromError(err) == pgxerrcode.InvalidCatalogName {
				// this error may happen if database is created in the last 5 minutes (`as of systemtime` points to a time before Genesis).
				// in this case we can ignore the database not found error and return with no usage.
				// if the database is really missing --> we have more serious problems than getting 0s from here.
				return nil
			}
			return err
		}

		for storageRows.Next() {
			var day time.Time
			var amount int64

			err = storageRows.Scan(&day, &amount)
			if err != nil {
				storageRows.Close()
				return err
			}

			storage = append(storage, accounting.ProjectUsageByDay{
				Date:  day.UTC(),
				Value: amount,
			})
		}

		storageRows.Close()
		err = storageRows.Err()
		if err != nil {
			return err
		}

		bandwidthRows, err := results.Query()
		if err != nil {
			return err
		}

		for bandwidthRows.Next() {
			var day time.Time
			var settled int64
			var allocated int64

			err = bandwidthRows.Scan(&day, &settled, &allocated)
			if err != nil {
				bandwidthRows.Close()
				return err
			}

			settledBandwidth = append(settledBandwidth, accounting.ProjectUsageByDay{
				Date:  day.UTC(),
				Value: settled,
			})

			allocatedBandwidth = append(allocatedBandwidth, accounting.ProjectUsageByDay{
				Date:  day.UTC(),
				Value: allocated,
			})
		}

		bandwidthRows.Close()
		err = bandwidthRows.Err()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, Error.New("unable to get project daily usage: %w", err)
	}

	return &accounting.ProjectDailyUsage{
		StorageUsage:            storage,
		AllocatedBandwidthUsage: allocatedBandwidth,
		SettledBandwidthUsage:   settledBandwidth,
	}, nil
}

// DeleteProjectBandwidthBefore deletes project bandwidth rollups before the given time.
func (db *ProjectAccounting) DeleteProjectBandwidthBefore(ctx context.Context, before time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.DB.ExecContext(ctx, db.db.Rebind("DELETE FROM project_bandwidth_daily_rollups WHERE interval_day < ?"), before)

	return err
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

// UpdateProjectBandwidthLimit updates project bandwidth limit.
func (db *ProjectAccounting) UpdateProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID, limit memory.Size) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
		dbx.Project_Update_Fields{
			BandwidthLimit: dbx.Project_BandwidthLimit(limit.Int64()),
		},
	)

	return err
}

// UpdateProjectSegmentLimit updates project segment limit.
func (db *ProjectAccounting) UpdateProjectSegmentLimit(ctx context.Context, projectID uuid.UUID, limit int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
		dbx.Project_Update_Fields{
			SegmentLimit: dbx.Project_SegmentLimit(limit),
		},
	)

	return err
}

// GetProjectStorageLimit returns project storage usage limit.
func (db *ProjectAccounting) GetProjectStorageLimit(ctx context.Context, projectID uuid.UUID) (_ *int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_Project_UsageLimit_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
	)
	if err != nil {
		return nil, err
	}

	return row.UsageLimit, nil
}

// GetProjectBandwidthLimit returns project bandwidth usage limit.
func (db *ProjectAccounting) GetProjectBandwidthLimit(ctx context.Context, projectID uuid.UUID) (_ *int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_Project_BandwidthLimit_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
	)
	if err != nil {
		return nil, err
	}

	return row.BandwidthLimit, nil
}

// GetProjectObjectsSegments returns project objects and segments number.
func (db *ProjectAccounting) GetProjectObjectsSegments(ctx context.Context, projectID uuid.UUID) (objectsSegments accounting.ProjectObjectsSegments, err error) {
	defer mon.Task()(&ctx)(&err)

	var latestDate time.Time
	latestDateRow := db.db.QueryRowContext(ctx, db.db.Rebind(`
		SELECT interval_start FROM bucket_storage_tallies bst
		WHERE
			project_id = ?
			AND EXISTS (SELECT 1 FROM bucket_metainfos bm WHERE bm.project_id = bst.project_id)
		ORDER BY interval_start DESC
		LIMIT 1
	`), projectID[:])
	if err = latestDateRow.Scan(&latestDate); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return accounting.ProjectObjectsSegments{}, nil
		}
		return accounting.ProjectObjectsSegments{}, err
	}

	// calculate total segments and objects count.
	storageTalliesRows := db.db.QueryRowContext(ctx, db.db.Rebind(`
		SELECT
			SUM(total_segments_count),
			SUM(object_count)
		FROM
			bucket_storage_tallies
		WHERE
			project_id = ? AND
			interval_start = ?
	`), projectID[:], latestDate)
	if err = storageTalliesRows.Scan(&objectsSegments.SegmentCount, &objectsSegments.ObjectCount); err != nil {
		return accounting.ProjectObjectsSegments{}, err
	}

	return objectsSegments, nil
}

// GetProjectSegmentLimit returns project segment limit.
func (db *ProjectAccounting) GetProjectSegmentLimit(ctx context.Context, projectID uuid.UUID) (_ *int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_Project_SegmentLimit_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
	)
	if err != nil {
		return nil, err
	}

	return row.SegmentLimit, nil
}

// GetProjectTotal retrieves project usage for a given period.
func (db *ProjectAccounting) GetProjectTotal(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ *accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	usages, err := db.GetProjectTotalByPartner(ctx, projectID, nil, since, before)
	if err != nil {
		return nil, err
	}
	if usage, ok := usages[""]; ok {
		return &usage, nil
	}
	return &accounting.ProjectUsage{Since: since, Before: before}, nil
}

// GetProjectTotalByPartner retrieves project usage for a given period categorized by partner name.
// Unpartnered usage or usage for a partner not present in partnerNames is mapped to the empty string.
func (db *ProjectAccounting) GetProjectTotalByPartner(ctx context.Context, projectID uuid.UUID, partnerNames []string, since, before time.Time) (usages map[string]accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since)
	bucketNames, err := db.getBucketsSinceAndBefore(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	storageQuery := db.db.Rebind(`
		SELECT
			bucket_storage_tallies.interval_start,
			bucket_storage_tallies.total_bytes,
			bucket_storage_tallies.inline,
			bucket_storage_tallies.remote,
			bucket_storage_tallies.total_segments_count,
			bucket_storage_tallies.object_count
		FROM
			bucket_storage_tallies
		WHERE
			bucket_storage_tallies.project_id = ? AND
			bucket_storage_tallies.bucket_name = ? AND
			bucket_storage_tallies.interval_start >= ? AND
			bucket_storage_tallies.interval_start < ?
		ORDER BY bucket_storage_tallies.interval_start DESC
	`)

	totalEgressQuery := db.db.Rebind(`
		SELECT
			COALESCE(SUM(settled) + SUM(inline), 0)
		FROM
			bucket_bandwidth_rollups
		WHERE
			project_id = ? AND
			bucket_name = ? AND
			interval_start >= ? AND
			interval_start < ? AND
			action = ?;
	`)

	usages = make(map[string]accounting.ProjectUsage)

	for _, bucket := range bucketNames {
		valueAttr, err := db.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
			dbx.ValueAttribution_ProjectId(projectID[:]),
			dbx.ValueAttribution_BucketName([]byte(bucket)))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		var partner string
		if valueAttr != nil && valueAttr.UserAgent != nil {
			partner, err = tryFindPartnerByUserAgent(valueAttr.UserAgent, partnerNames)
			if err != nil {
				return nil, err
			}
		}
		if _, ok := usages[partner]; !ok {
			usages[partner] = accounting.ProjectUsage{Since: since, Before: before}
		}
		usage := usages[partner]

		storageTalliesRows, err := db.db.QueryContext(ctx, storageQuery, projectID[:], []byte(bucket), since, before)
		if err != nil {
			return nil, err
		}

		var prevTally *accounting.BucketStorageTally
		for storageTalliesRows.Next() {
			tally := accounting.BucketStorageTally{}

			var inline, remote int64
			err = storageTalliesRows.Scan(&tally.IntervalStart, &tally.TotalBytes, &inline, &remote, &tally.TotalSegmentCount, &tally.ObjectCount)
			if err != nil {
				return nil, errs.Combine(err, storageTalliesRows.Close())
			}
			if tally.TotalBytes == 0 {
				tally.TotalBytes = inline + remote
			}

			if prevTally == nil {
				prevTally = &tally
				continue
			}

			hours := prevTally.IntervalStart.Sub(tally.IntervalStart).Hours()
			usage.Storage += memory.Size(tally.TotalBytes).Float64() * hours
			usage.SegmentCount += float64(tally.TotalSegmentCount) * hours
			usage.ObjectCount += float64(tally.ObjectCount) * hours

			prevTally = &tally
		}

		err = errs.Combine(storageTalliesRows.Err(), storageTalliesRows.Close())
		if err != nil {
			return nil, err
		}

		totalEgressRow := db.db.QueryRowContext(ctx, totalEgressQuery, projectID[:], []byte(bucket), since, before, pb.PieceAction_GET)
		if err != nil {
			return nil, err
		}

		var egress int64
		if err = totalEgressRow.Scan(&egress); err != nil {
			return nil, err
		}
		usage.Egress += egress

		usages[partner] = usage
	}

	// We search for project user_agent for cases when buckets haven't been created yet.
	if len(usages) == 0 {
		userAgentRow, err := db.db.Get_Project_UserAgent_By_Id(ctx, dbx.Project_Id(projectID[:]))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if userAgentRow != nil && userAgentRow.UserAgent != nil {
			partner, err := tryFindPartnerByUserAgent(userAgentRow.UserAgent, partnerNames)
			if err != nil {
				return nil, err
			}
			usages[partner] = accounting.ProjectUsage{}
		}
	}

	return usages, nil
}

func tryFindPartnerByUserAgent(userAgent []byte, partnerNames []string) (string, error) {
	entries, err := useragent.ParseEntries(userAgent)
	if err != nil {
		return "", err
	}

	var partner string
	if len(entries) != 0 {
		for _, iterPartner := range partnerNames {
			if entries[0].Product == iterPartner {
				partner = iterPartner
				break
			}
		}
	}

	return partner, nil
}

// GetBucketUsageRollups retrieves summed usage rollups for every bucket of particular project for a given period.
func (db *ProjectAccounting) GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ []accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since.UTC())
	before = before.UTC()

	buckets, err := db.getBucketsSinceAndBefore(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	var bucketUsageRollups []accounting.BucketUsageRollup
	for _, bucket := range buckets {
		bucketRollup, err := db.getSingleBucketRollup(ctx, projectID, bucket, since, before)
		if err != nil {
			return nil, err
		}

		bucketUsageRollups = append(bucketUsageRollups, *bucketRollup)
	}

	return bucketUsageRollups, nil
}

// GetSingleBucketUsageRollup retrieves usage rollup for a single bucket of particular project for a given period.
func (db *ProjectAccounting) GetSingleBucketUsageRollup(ctx context.Context, projectID uuid.UUID, bucket string, since, before time.Time) (_ *accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since.UTC())
	before = before.UTC()

	bucketRollup, err := db.getSingleBucketRollup(ctx, projectID, bucket, since, before)
	if err != nil {
		return nil, err
	}

	return bucketRollup, nil
}

func (db *ProjectAccounting) getSingleBucketRollup(ctx context.Context, projectID uuid.UUID, bucket string, since, before time.Time) (*accounting.BucketUsageRollup, error) {
	rollupsQuery := db.db.Rebind(`SELECT SUM(settled), SUM(inline), action
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		GROUP BY action`)

	// TODO: should be optimized
	storageQuery := db.db.All_BucketStorageTally_By_ProjectId_And_BucketName_And_IntervalStart_GreaterOrEqual_And_IntervalStart_LessOrEqual_OrderBy_Desc_IntervalStart

	bucketRollup := &accounting.BucketUsageRollup{
		ProjectID:  projectID,
		BucketName: bucket,
		Since:      since,
		Before:     before,
	}

	// get bucket_bandwidth_rollup
	rollupRows, err := db.db.QueryContext(ctx, rollupsQuery, projectID[:], []byte(bucket), since, before)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rollupRows.Close()) }()

	// fill egress
	for rollupRows.Next() {
		var action pb.PieceAction
		var settled, inline int64

		err = rollupRows.Scan(&settled, &inline, &action)
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
	if err := rollupRows.Err(); err != nil {
		return nil, err
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

		if current.TotalBytes > 0 {
			bucketRollup.TotalStoredData += memory.Size(current.TotalBytes).GB() * hours
		} else {
			bucketRollup.TotalStoredData += memory.Size(current.Remote+current.Inline).GB() * hours
		}
		bucketRollup.MetadataSize += memory.Size(current.MetadataSize).GB() * hours
		if current.TotalSegmentsCount > 0 {
			bucketRollup.TotalSegments += float64(current.TotalSegmentsCount) * hours
		} else {
			bucketRollup.TotalSegments += float64(current.RemoteSegmentsCount+current.InlineSegmentsCount) * hours
		}
		bucketRollup.ObjectCount += float64(current.ObjectCount) * hours
	}

	return bucketRollup, nil
}

// prefixIncrement returns the lexicographically lowest byte string which is
// greater than origPrefix and does not have origPrefix as a prefix. If no such
// byte string exists (origPrefix is empty, or origPrefix contains only 0xff
// bytes), returns false for ok.
//
// examples: prefixIncrement([]byte("abc"))          -> ([]byte("abd", true)
//
//	prefixIncrement([]byte("ab\xff\xff"))   -> ([]byte("ac", true)
//	prefixIncrement([]byte(""))             -> (nil, false)
//	prefixIncrement([]byte("\x00"))         -> ([]byte("\x01", true)
//	prefixIncrement([]byte("\xff\xff\xff")) -> (nil, false)
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
	switch db.db.impl {
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
		return "", nil, errs.New("unhandled database: %v", db.db.driver)
	}

}

// GetBucketTotals retrieves bucket usage totals for period of time.
func (db *ProjectAccounting) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)
	bucketPrefix := []byte(cursor.Search)

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}
	if cursor.Page == 0 {
		return nil, errs.New("page can not be 0")
	}

	page := &accounting.BucketUsagePage{
		Search: cursor.Search,
		Limit:  cursor.Limit,
		Offset: uint64((cursor.Page - 1) * cursor.Limit),
	}

	bucketNameRange, incrPrefix, err := db.prefixMatch("name", bucketPrefix)
	if err != nil {
		return nil, err
	}
	countQuery := db.db.Rebind(`SELECT COUNT(name) FROM bucket_metainfos
	WHERE project_id = ? AND ` + bucketNameRange)

	args := []interface{}{
		projectID[:],
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

	bucketsQuery := db.db.Rebind(`SELECT name, versioning, placement, created_at FROM bucket_metainfos
	WHERE project_id = ? AND ` + bucketNameRange + `ORDER BY name ASC LIMIT ? OFFSET ?`)

	args = []interface{}{
		projectID[:],
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

	type bucketWithCreationDate struct {
		name       string
		versioning satbuckets.Versioning
		placement  storj.PlacementConstraint
		createdAt  time.Time
	}

	var buckets []bucketWithCreationDate
	for bucketRows.Next() {
		var (
			bucket     string
			versioning satbuckets.Versioning
			placement  storj.PlacementConstraint
			createdAt  time.Time
		)
		err = bucketRows.Scan(&bucket, &versioning, &placement, &createdAt)
		if err != nil {
			return nil, err
		}

		buckets = append(buckets, bucketWithCreationDate{
			name:       bucket,
			versioning: versioning,
			placement:  placement,
			createdAt:  createdAt,
		})
	}
	if err := bucketRows.Err(); err != nil {
		return nil, err
	}

	rollupsQuery := db.db.Rebind(`SELECT COALESCE(SUM(settled) + SUM(inline), 0)
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ? AND action = ?`)

	storageQuery := db.db.Rebind(`SELECT total_bytes, inline, remote, object_count, total_segments_count
		FROM bucket_storage_tallies
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		ORDER BY interval_start DESC
		LIMIT 1`)

	var bucketUsages []accounting.BucketUsage
	for _, bucket := range buckets {
		bucketUsage := accounting.BucketUsage{
			ProjectID:        projectID,
			BucketName:       bucket.name,
			Versioning:       bucket.versioning,
			DefaultPlacement: bucket.placement,
			Since:            bucket.createdAt,
			Before:           before,
		}

		// get bucket_bandwidth_rollups
		rollupRow := db.db.QueryRowContext(ctx, rollupsQuery, projectID[:], []byte(bucket.name), bucket.createdAt, before, pb.PieceAction_GET)

		var egress int64
		err = rollupRow.Scan(&egress)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}

		bucketUsage.Egress = memory.Size(egress).GB()

		storageRow := db.db.QueryRowContext(ctx, storageQuery, projectID[:], []byte(bucket.name), bucket.createdAt, before)

		var tally accounting.BucketStorageTally
		var inline, remote int64
		err = storageRow.Scan(&tally.TotalBytes, &inline, &remote, &tally.ObjectCount, &tally.TotalSegmentCount)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}

		if tally.TotalBytes == 0 {
			tally.TotalBytes = inline + remote
		}

		// fill storage and object count
		bucketUsage.Storage = memory.Size(tally.Bytes()).GB()
		bucketUsage.SegmentCount = tally.TotalSegmentCount
		bucketUsage.ObjectCount = tally.ObjectCount

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

// ArchiveRollupsBefore archives rollups older than a given time.
func (db *ProjectAccounting) ArchiveRollupsBefore(ctx context.Context, before time.Time, batchSize int) (archivedCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	if batchSize <= 0 {
		return 0, nil
	}

	switch db.db.impl {
	case dbutil.Cockroach:

		// We operate one action at a time, because we have an index on `(action, interval_start, project_id)`.
		for action := range pb.PieceAction_name {
			count, err := db.archiveRollupsBeforeByAction(ctx, action, before, batchSize)
			archivedCount += count
			if err != nil {
				return archivedCount, Error.Wrap(err)
			}
		}
		return archivedCount, nil
	case dbutil.Postgres:
		err := db.db.DB.QueryRow(ctx, `
			WITH rollups_to_move AS (
				DELETE FROM bucket_bandwidth_rollups
				WHERE interval_start <= $1
				RETURNING *
			), moved_rollups AS (
				INSERT INTO bucket_bandwidth_rollup_archives(bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
				SELECT bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled FROM rollups_to_move
				RETURNING *
			)
			SELECT count(*) FROM moved_rollups
		`, before).Scan(&archivedCount)
		return archivedCount, Error.Wrap(err)
	default:
		return 0, nil
	}
}

func (db *ProjectAccounting) archiveRollupsBeforeByAction(ctx context.Context, action int32, before time.Time, batchSize int) (archivedCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		var rowCount int
		err := db.db.QueryRow(ctx, `
			WITH rollups_to_move AS (
				DELETE FROM bucket_bandwidth_rollups
				WHERE action = $1 AND interval_start <= $2
				LIMIT $3 RETURNING *
			), moved_rollups AS (
				INSERT INTO bucket_bandwidth_rollup_archives(bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
				SELECT bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled FROM rollups_to_move
				RETURNING *
			)
			SELECT count(*) FROM moved_rollups
		`, int(action), before, batchSize).Scan(&rowCount)
		if err != nil {
			return archivedCount, Error.Wrap(err)
		}
		archivedCount += rowCount

		if rowCount < batchSize {
			return archivedCount, nil
		}
	}
}

// getBucketsSinceAndBefore lists distinct bucket names for a project within a specific timeframe.
func (db *ProjectAccounting) getBucketsSinceAndBefore(ctx context.Context, projectID uuid.UUID, since, before time.Time) (buckets []string, err error) {
	defer mon.Task()(&ctx)(&err)

	queryFormat := `SELECT DISTINCT bucket_name
		FROM %s
		WHERE project_id = ?
		AND interval_start >= ?
		AND interval_start < ?`

	bucketMap := make(map[string]struct{})

	for _, tableName := range []string{"bucket_storage_tallies", "bucket_bandwidth_rollups"} {
		query := db.db.Rebind(fmt.Sprintf(queryFormat, tableName))

		rows, err := db.db.QueryContext(ctx, query, projectID[:], since, before)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var bucket string
			err = rows.Scan(&bucket)
			if err != nil {
				return nil, errs.Combine(err, rows.Close())
			}
			bucketMap[bucket] = struct{}{}
		}

		err = errs.Combine(rows.Err(), rows.Close())
		if err != nil {
			return nil, err
		}
	}

	for bucket := range bucketMap {
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

// timeTruncateDown truncates down to the hour before to be in sync with orders endpoint.
func timeTruncateDown(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// GetProjectLimits returns all project limits including user specified usage and bandwidth limits.
func (db *ProjectAccounting) GetProjectLimits(ctx context.Context, projectID uuid.UUID) (_ accounting.ProjectLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_Project_BandwidthLimit_Project_UserSpecifiedBandwidthLimit_Project_UsageLimit_Project_UserSpecifiedUsageLimit_Project_SegmentLimit_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_By_Id(ctx,
		dbx.Project_Id(projectID[:]),
	)
	if err != nil {
		return accounting.ProjectLimits{}, err
	}

	return accounting.ProjectLimits{
		Usage:            row.UsageLimit,
		UserSetUsage:     row.UserSpecifiedUsageLimit,
		Bandwidth:        row.BandwidthLimit,
		UserSetBandwidth: row.UserSpecifiedBandwidthLimit,
		Segments:         row.SegmentLimit,

		RateLimit:        row.RateLimit,
		BurstLimit:       row.BurstLimit,
		RateLimitHead:    row.RateLimitHead,
		BurstLimitHead:   row.BurstLimitHead,
		RateLimitGet:     row.RateLimitGet,
		BurstLimitGet:    row.BurstLimitGet,
		RateLimitPut:     row.RateLimitPut,
		BurstLimitPut:    row.BurstLimitPut,
		RateLimitList:    row.RateLimitList,
		BurstLimitList:   row.BurstLimitList,
		RateLimitDelete:  row.RateLimitDel,
		BurstLimitDelete: row.BurstLimitDel,
	}, nil
}

// GetRollupsSince retrieves all archived rollup records since a given time.
func (db *ProjectAccounting) GetRollupsSince(ctx context.Context, since time.Time) (bwRollups []orders.BucketBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_BucketBandwidthRollup_By_IntervalStart_GreaterOrEqual_Continuation
	for {
		dbxRollups, next, err := db.db.Paged_BucketBandwidthRollup_By_IntervalStart_GreaterOrEqual(ctx,
			dbx.BucketBandwidthRollup_IntervalStart(since),
			pageLimit, cursor)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		cursor = next
		for _, dbxRollup := range dbxRollups {
			projectID, err := uuid.FromBytes(dbxRollup.ProjectId)
			if err != nil {
				return nil, err
			}
			bwRollups = append(bwRollups, orders.BucketBandwidthRollup{
				ProjectID:  projectID,
				BucketName: string(dbxRollup.BucketName),
				Action:     pb.PieceAction(dbxRollup.Action),
				Inline:     int64(dbxRollup.Inline),
				Allocated:  int64(dbxRollup.Allocated),
				Settled:    int64(dbxRollup.Settled),
			})
		}
		if cursor == nil {
			return bwRollups, nil
		}
	}
}

// GetArchivedRollupsSince retrieves all archived rollup records since a given time.
func (db *ProjectAccounting) GetArchivedRollupsSince(ctx context.Context, since time.Time) (bwRollups []orders.BucketBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	pageLimit := db.db.opts.ReadRollupBatchSize
	if pageLimit <= 0 {
		pageLimit = 10000
	}

	var cursor *dbx.Paged_BucketBandwidthRollupArchive_By_IntervalStart_GreaterOrEqual_Continuation
	for {
		dbxRollups, next, err := db.db.Paged_BucketBandwidthRollupArchive_By_IntervalStart_GreaterOrEqual(ctx,
			dbx.BucketBandwidthRollupArchive_IntervalStart(since),
			pageLimit, cursor)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		cursor = next
		for _, dbxRollup := range dbxRollups {
			projectID, err := uuid.FromBytes(dbxRollup.ProjectId)
			if err != nil {
				return nil, err
			}
			bwRollups = append(bwRollups, orders.BucketBandwidthRollup{
				ProjectID:  projectID,
				BucketName: string(dbxRollup.BucketName),
				Action:     pb.PieceAction(dbxRollup.Action),
				Inline:     int64(dbxRollup.Inline),
				Allocated:  int64(dbxRollup.Allocated),
				Settled:    int64(dbxRollup.Settled),
			})
		}
		if cursor == nil {
			return bwRollups, nil
		}
	}
}
