// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
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
	"storj.io/storj/shared/dbutil/spannerutil"
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
	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
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
	case dbutil.Spanner:
		type bucketTally struct {
			BucketName          []byte
			ProjectID           []byte
			TotalBytes          int64
			Inline              int64
			Remote              int64
			TotalSegmentsCount  int64
			RemoteSegmentsCount int64
			InlineSegmentsCount int64
			ObjectCount         int64
			MetadataSize        int64
		}

		var insertBucketTallies []bucketTally
		for _, info := range bucketTallies {
			insertBucketTallies = append(insertBucketTallies, bucketTally{
				BucketName:          []byte(info.BucketName),
				ProjectID:           info.ProjectID[:],
				TotalBytes:          info.TotalBytes,
				Inline:              0,
				Remote:              0,
				TotalSegmentsCount:  info.TotalSegments,
				RemoteSegmentsCount: 0,
				InlineSegmentsCount: 0,
				ObjectCount:         info.ObjectCount,
				MetadataSize:        info.MetadataSize,
			})
		}

		query := `
		INSERT INTO bucket_storage_tallies (
			interval_start,
			bucket_name,
			project_id,
			total_bytes,
			inline,
			remote,
			total_segments_count,
			remote_segments_count,
			inline_segments_count,
			object_count,
			metadata_size
		)
		SELECT
			?,
			BucketName,
			ProjectID,
			TotalBytes,
			Inline,
			Remote,
			TotalSegmentsCount,
			RemoteSegmentsCount,
			InlineSegmentsCount,
			ObjectCount,
			MetadataSize,
		FROM UNNEST(?) AS bucket_name
       `
		_, err = db.db.ExecContext(
			ctx,
			query,
			intervalStart,
			insertBucketTallies,
		)
		return Error.Wrap(err)
	default:
		return Error.New("unsupported database: %v", db.db.impl)
	}
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
				BucketName: metabase.BucketName(dbxTally.BucketName),
			},
			ObjectCount:   int64(dbxTally.ObjectCount),
			TotalSegments: int64(totalSegments),
			TotalBytes:    int64(totalBytes),
			MetadataSize:  int64(dbxTally.MetadataSize),
		})
	}

	return tallies, nil
}

// DeleteTalliesBefore deletes tallies with an interval start before the given time.
//
// Spanner implementation returns an estimated count of the number of rows deleted.
// The actual number of affected rows may be greater than the estimate.
func (db *ProjectAccounting) DeleteTalliesBefore(ctx context.Context, before time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		return db.db.Delete_BucketStorageTally_By_IntervalStart_Less(ctx, dbx.BucketStorageTally_IntervalStart(before))
	case dbutil.Spanner:
		var count int64
		return count, spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) error {
			statement := spanner.Statement{
				SQL: `DELETE FROM bucket_storage_tallies WHERE bucket_storage_tallies.interval_start < @before`,
				Params: map[string]interface{}{
					"before": before,
				},
			}
			var err error
			count, err = client.PartitionedUpdateWithOptions(ctx, statement, spanner.QueryOptions{
				Priority: spannerpb.RequestOptions_PRIORITY_LOW,
			})
			return err
		})
	default:
		return 0, errs.New("unsupported database dialect: %s", db.db.impl)
	}
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

// GetPreviouslyNonEmptyTallyBucketsInRange returns a list of bucket locations within the given range
// whose most recent tally does not represent empty usage.
func (db *ProjectAccounting) GetPreviouslyNonEmptyTallyBucketsInRange(ctx context.Context, from, to metabase.BucketLocation, asOfSystemInterval time.Duration) (result []metabase.BucketLocation, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		// it constantly bothers me that there isn't a better query
		// for this class of problem. problem: i want another value in the
		// row that has the max value within a given group!
		// see https://stackoverflow.com/questions/12102200/get-records-with-max-value-for-each-group-of-grouped-sql-results
		// for a list of people banging their heads against the
		// wall (the highest voted answer is an O(n^2) query!).
		rows, err = db.db.QueryContext(ctx, `
		SELECT project_id, bucket_name
		FROM (
			SELECT project_id, bucket_name
			FROM bucket_storage_tallies
			WHERE (project_id, bucket_name) BETWEEN ($1, $2) AND ($3, $4)
			GROUP BY project_id, bucket_name
		) bm`+
			db.db.impl.AsOfSystemInterval(asOfSystemInterval)+
			` WHERE NOT 0 IN (
			SELECT object_count FROM bucket_storage_tallies
			WHERE (project_id, bucket_name) = (bm.project_id, bm.bucket_name)
			ORDER BY interval_start DESC
			LIMIT 1
		)
		`, from.ProjectID, []byte(from.BucketName), to.ProjectID, []byte(to.BucketName))
	case dbutil.Spanner:
		var fromTuple string
		var toTuple string

		fromTuple, err = spannerutil.TupleGreaterThanSQL([]string{"project_id", "bucket_name"}, []string{"@from_project_id", "@from_name"}, true)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		toTuple, err = spannerutil.TupleGreaterThanSQL([]string{"@to_project_id", "@to_name"}, []string{"project_id", "bucket_name"}, true)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		rows, err = db.db.QueryContext(ctx, `
			SELECT project_id, bucket_name FROM (
				SELECT
					project_id,
					bucket_name,
					ANY_VALUE(object_count HAVING MAX interval_start) AS last_object_count
				FROM
					bucket_storage_tallies
				WHERE `+fromTuple+` AND `+toTuple+`
				GROUP BY
					project_id,
					bucket_name
				HAVING
					last_object_count > 0
			)
		`, sql.Named("from_project_id", from.ProjectID), sql.Named("from_name", []byte(from.BucketName)), sql.Named("to_project_id", to.ProjectID), sql.Named("to_name", []byte(to.BucketName)))
	default:
		return nil, errs.New("unsupported database dialect: %s", db.db.impl)
	}
	err = withRows(rows, err)(func(r tagsql.Rows) error {
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

// GetBucketsWithEntitlementsInRange returns all buckets in a given range with their entitlements.
func (db *ProjectAccounting) GetBucketsWithEntitlementsInRange(
	ctx context.Context,
	from, to metabase.BucketLocation,
	projectScopePrefix string,
) ([]accounting.BucketLocationWithEntitlements, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		return db.getBucketsWithEntitlementsPostgres(ctx, from, to, projectScopePrefix)
	case dbutil.Spanner:
		return db.getBucketsWithEntitlementsSpanner(ctx, from, to, projectScopePrefix)
	default:
		return nil, Error.New("unsupported database dialect: %s", db.db.impl)
	}
}

func (db *ProjectAccounting) getBucketsWithEntitlementsPostgres(
	ctx context.Context,
	from, to metabase.BucketLocation,
	projectScopePrefix string,
) (result []accounting.BucketLocationWithEntitlements, err error) {
	query := `
		SELECT
			bm.project_id,
			bm.name,
			bm.placement,
			e.features,
			COALESCE(latest_tally.object_count, 0) > 0 AS has_previous_tally
		FROM bucket_metainfos bm
		LEFT JOIN (
			SELECT DISTINCT ON (bst.project_id, bst.bucket_name)
				bst.project_id,
				bst.bucket_name,
				bst.object_count
			FROM bucket_storage_tallies bst
			WHERE (bst.project_id, bst.bucket_name) BETWEEN ($1, $2) AND ($3, $4)
			ORDER BY bst.project_id, bst.bucket_name, bst.interval_start DESC
		) latest_tally
			ON bm.project_id = latest_tally.project_id
			AND bm.name = latest_tally.bucket_name
		INNER JOIN projects p
			ON bm.project_id = p.id
		LEFT JOIN entitlements e
			ON e.scope = $5::bytea || p.public_id
		WHERE (bm.project_id, bm.name) BETWEEN ($1, $2) AND ($3, $4)
	`

	rows, err := db.db.QueryContext(ctx, query,
		from.ProjectID, []byte(from.BucketName),
		to.ProjectID, []byte(to.BucketName),
		[]byte(projectScopePrefix))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = withRows(rows, err)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var b accounting.BucketLocationWithEntitlements
			var features sql.NullString
			if err := rows.Scan(
				&b.Location.ProjectID,
				&b.Location.BucketName,
				&b.Placement,
				&features,
				&b.HasPreviousTally,
			); err != nil {
				return Error.Wrap(err)
			}
			if features.Valid {
				if err = json.Unmarshal([]byte(features.String), &b.ProjectFeatures); err != nil {
					return Error.Wrap(err)
				}
			}
			result = append(result, b)
		}
		return nil
	})

	return result, Error.Wrap(err)
}

func (db *ProjectAccounting) getBucketsWithEntitlementsSpanner(
	ctx context.Context,
	from, to metabase.BucketLocation,
	projectScopePrefix string,
) (result []accounting.BucketLocationWithEntitlements, err error) {
	fromTupleTally, err := spannerutil.TupleGreaterThanSQL([]string{"project_id", "bucket_name"}, []string{"@from_project_id", "@from_name"}, true)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	toTupleTally, err := spannerutil.TupleGreaterThanSQL([]string{"@to_project_id", "@to_name"}, []string{"project_id", "bucket_name"}, true)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	fromTupleBucket, err := spannerutil.TupleGreaterThanSQL([]string{"bm.project_id", "bm.name"}, []string{"@from_project_id", "@from_name"}, true)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	toTupleBucket, err := spannerutil.TupleGreaterThanSQL([]string{"@to_project_id", "@to_name"}, []string{"bm.project_id", "bm.name"}, true)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	query := `
		SELECT
			bm.project_id,
			bm.name,
			bm.placement,
			TO_JSON_STRING(e.features) AS features_json,
			COALESCE(bucket_info.last_object_count, 0) > 0 AS has_previous_tally
		FROM bucket_metainfos bm
		LEFT JOIN (
			SELECT
				project_id,
				bucket_name,
				ANY_VALUE(object_count HAVING MAX interval_start) AS last_object_count
			FROM bucket_storage_tallies
			WHERE ` + fromTupleTally + ` AND ` + toTupleTally + `
			GROUP BY project_id, bucket_name
		) bucket_info
			ON bm.project_id = bucket_info.project_id
			AND bm.name = bucket_info.bucket_name
		INNER JOIN projects p
			ON bm.project_id = p.id
		LEFT JOIN entitlements e
			ON e.scope = CONCAT(@project_scope_prefix, p.public_id)
		WHERE ` + fromTupleBucket + ` AND ` + toTupleBucket + `
	`

	rows, err := db.db.QueryContext(ctx, query,
		sql.Named("from_project_id", from.ProjectID),
		sql.Named("from_name", []byte(from.BucketName)),
		sql.Named("to_project_id", to.ProjectID),
		sql.Named("to_name", []byte(to.BucketName)),
		sql.Named("project_scope_prefix", []byte(projectScopePrefix)))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = withRows(rows, err)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var b accounting.BucketLocationWithEntitlements
			var featuresJSON sql.NullString

			if err = rows.Scan(
				&b.Location.ProjectID,
				&b.Location.BucketName,
				&b.Placement,
				&featuresJSON,
				&b.HasPreviousTally,
			); err != nil {
				return Error.Wrap(err)
			}

			if raw, ok := hackyResolveSpannerJSONColumn(featuresJSON); ok && len(raw) > 0 {
				if err = json.Unmarshal(raw, &b.ProjectFeatures); err != nil {
					return Error.Wrap(err)
				}
			}

			result = append(result, b)
		}
		return nil
	})

	return result, Error.Wrap(err)
}

// GetProjectSettledBandwidthTotal returns the sum of GET bandwidth usage settled for a projectID in the past time frame.
func (db *ProjectAccounting) GetProjectSettledBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum *int64
	// action uses int64 for compatibility with Spanner as Spanner does not support int32
	actionGet := int64(pb.PieceAction_GET)
	query := `SELECT SUM(settled) FROM bucket_bandwidth_rollups WHERE project_id = ? AND action = ? AND interval_start >= ?;`
	err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID[:], actionGet, from.UTC()).Scan(&sum)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && sum == nil) {
		return 0, nil
	}
	if err != nil {
		return 0, errs.Wrap(err)
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
	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), expiredSince, projectID[:], startOfMonth, periodEnd).Scan(&egress)
	case dbutil.Spanner:
		expiredSinceCivil := civil.DateOf(expiredSince)
		startOfMonthCivil := civil.DateOf(startOfMonth)
		periodEndCivil := civil.DateOf(periodEnd)
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), expiredSinceCivil, projectID[:], startOfMonthCivil, periodEndCivil).Scan(&egress)
	default:
		return 0, errs.New("unsupported database dialect: %s", db.db.impl)
	}
	if errors.Is(err, sql.ErrNoRows) || (err == nil && egress == nil) {
		return 0, nil
	}
	if err != nil {
		return 0, Error.Wrap(err)
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
	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID[:], startOfMonth, periodEnd).Scan(&egress)
	case dbutil.Spanner:
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID[:], civil.DateOf(startOfMonth), civil.DateOf(periodEnd)).Scan(&egress)
	default:
		return 0, errs.New("unsupported database dialect: %s", db.db.impl)
	}
	if errors.Is(err, sql.ErrNoRows) || (err == nil && egress == nil) {
		return 0, nil
	}
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return *egress, err
}

// GetProjectDailyBandwidth returns project bandwidth (allocated and settled) for the specified day.
func (db *ProjectAccounting) GetProjectDailyBandwidth(ctx context.Context, projectID uuid.UUID, year int, month time.Month, day int) (allocated int64, settled, dead int64, err error) {
	defer mon.Task()(&ctx)(&err)

	startOfMonth := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	query := `SELECT egress_allocated, egress_settled, egress_dead FROM project_bandwidth_daily_rollups WHERE project_id = ? AND interval_day = ?;`
	switch db.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID.Bytes(), startOfMonth).Scan(&allocated, &settled, &dead)
	case dbutil.Spanner:
		err = db.db.QueryRowContext(ctx, db.db.Rebind(query), projectID.Bytes(), civil.DateOf(startOfMonth)).Scan(&allocated, &settled, &dead)
	default:
		return 0, 0, 0, errs.New("unsupported database dialect: %s", db.db.impl)
	}
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
	expiredSince := nowBeginningOfDay.Add(time.Duration(-allocatedExpirationInDays) * time.Hour * 24)

	allocatedBandwidth := make([]accounting.ProjectUsageByDay, 0)
	settledBandwidth := make([]accounting.ProjectUsageByDay, 0)
	storage := make([]accounting.ProjectUsageByDay, 0)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		err = pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
			var batch pgx.Batch

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
	case dbutil.Spanner:
		err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			// TODO(spanner): remove TIMESTAMP_TRUNC, when spanner emulator gets fixed
			//
			// We need to do TIMESTAMP_TRUNC(interval_start, MICROSECOND, 'UTC') as interval_start
			// To ensure that MAX(interval_start) == interval_start
			//
			// See https://github.com/GoogleCloudPlatform/cloud-spanner-emulator/issues/73 for details.

			storageQuery := `
			WITH
				project_usage AS (
					SELECT
						TIMESTAMP_TRUNC(interval_start, MICROSECOND, 'UTC') as interval_start,
						CAST(interval_start AS DATE) AS interval_day,
						project_id,
						bucket_name,
						total_bytes
					FROM bucket_storage_tallies
					WHERE
						project_id = ? AND
						interval_start >= ? AND
						interval_start <= ?
				),
				project_usage_distinct AS (
					SELECT
						project_id,
						bucket_name,
						MAX(interval_start) AS interval_start
					FROM project_usage
					GROUP BY
						project_id,
						bucket_name,
						interval_day
				)
			-- Sum all buckets usage in the same project.
			SELECT
				interval_day,
				SUM(total_bytes) AS total_bytes
			FROM (
				SELECT
					project_id,
					bucket_name,
					total_bytes,
					interval_day,
					interval_start
				FROM project_usage
				WHERE
					(bucket_name, project_id, interval_start) IN (SELECT (bucket_name, project_id, interval_start) FROM project_usage_distinct)
			)
			GROUP BY
				project_id,
				interval_day`

			storageRows, err := tx.QueryContext(ctx, storageQuery, projectID, fromBeginningOfDay, toEndOfDay)
			if err != nil {
				return Error.Wrap(err)
			}

			for storageRows.Next() {
				var day civil.Date
				var amount int64

				if err := storageRows.Scan(&day, &amount); err != nil {
					err = errs.Combine(err, storageRows.Close())
					return err
				}

				storage = append(storage, accounting.ProjectUsageByDay{
					Date:  day.In(time.UTC),
					Value: amount,
				})
			}

			if err := errs.Combine(storageRows.Err(), storageRows.Close()); err != nil {
				return err
			}

			civilExpiredSince := civil.DateOf(expiredSince)
			civilFromBeginningOfDay := civil.DateOf(fromBeginningOfDay)
			civilToEndOfDay := civil.DateOf(toEndOfDay)

			bandwidthQuery := `
			SELECT interval_day, egress_settled,
				CASE WHEN interval_day < ?
				THEN egress_settled
				ELSE egress_allocated-egress_dead
					END AS allocated
			FROM project_bandwidth_daily_rollups
			WHERE project_id = ? AND (interval_day >= ? AND interval_day <= ?)`

			bandwidthRows, err := tx.QueryContext(ctx, bandwidthQuery, civilExpiredSince, projectID, civilFromBeginningOfDay, civilToEndOfDay)
			if err != nil {
				return Error.Wrap(err)
			}

			for bandwidthRows.Next() {
				var day civil.Date
				var settled int64
				var allocated int64

				if err := bandwidthRows.Scan(&day, &settled, &allocated); err != nil {
					err = errs.Combine(err, bandwidthRows.Close())
					return err
				}

				settledBandwidth = append(settledBandwidth, accounting.ProjectUsageByDay{
					Date:  day.In(time.UTC),
					Value: settled,
				})

				allocatedBandwidth = append(allocatedBandwidth, accounting.ProjectUsageByDay{
					Date:  day.In(time.UTC),
					Value: allocated,
				})
			}

			if err := errs.Combine(bandwidthRows.Err(), bandwidthRows.Close()); err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	default:
		return nil, errs.New("unsupported database dialect: %s", db.db.impl)
	}
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

	query := db.db.Rebind("DELETE FROM project_bandwidth_daily_rollups WHERE interval_day < ?")
	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		_, err = db.db.DB.ExecContext(ctx, query, before)
	case dbutil.Spanner:
		_, err = db.db.DB.ExecContext(ctx, query, civil.DateOf(before))
	default:
		return errs.New("unsupported database dialect: %s", db.db.impl)
	}

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
	usages, err := db.GetProjectTotalByPartnerAndPlacement(ctx, projectID, nil, since, before, true)
	if err != nil {
		return nil, err
	}
	if usage, ok := usages[""]; ok {
		return &usage, nil
	}
	return &accounting.ProjectUsage{Since: since, Before: before}, nil
}

// GetProjectTotalByPartnerAndPlacement retrieves project usage for a given period categorized by partner name and placement constraint.
// Unpartnered usage or usage for a partner not present in partnerNames is mapped to "|<placement>".
//
// If aggregate is true, the function ignores partners and placements and returns aggregated usage
// values with an empty string key instead of the partner|placement format.
//
// Storage tallies are calculated on intervals of two consecutive entries by multiplying the hours
// between the periods of the 2 entries by the bytes stored, number of segments, and number of
// objects.
// The consequences of calculating the storage tallies on intervals are:
//
//   - An interval with only one entry, will result in 0 storage (bytes, number of segments, and
//     number of objects)
//
//   - The method picks all the storage entries between since and before (excluded) plus the entry
//     with the time lower than since, but closes to since. The extra one is used as the lowest
//     boundary to start the calculation.
//
//     This translates that when calculating a monthly storage for example since is 1st of March and
//     before is 1st of April , it will include the last entry of February, but it won't calculate
//     the storage since the last period of March until April, which will be included in the next
//     monthly calculation (1st of April to 1st of May) as it happened with the last February entry
//     for this period.
func (db *ProjectAccounting) GetProjectTotalByPartnerAndPlacement(ctx context.Context, projectID uuid.UUID, partnerNames []string, since, before time.Time, aggregate bool) (usages map[string]accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since)
	buckets, err := db.GetBucketsSinceAndBefore(ctx, projectID, since, before, false)
	if err != nil {
		return nil, err
	}

	// we're going to get all tallies from the time range in question,
	// but we're also going to get the tally that comes immediately before
	// the time range in question, so we know how many hours to count the earliest
	// tally span within the timerange in question for.
	storageQuery := db.db.Rebind(`
		SELECT
			bst1.interval_start,
			bst1.total_bytes,
			bst1.inline,
			bst1.remote,
			bst1.total_segments_count,
			bst1.object_count
		FROM
			bucket_storage_tallies bst1
		WHERE
			bst1.project_id = ? AND
			bst1.bucket_name = ? AND
			(
				(
					bst1.interval_start >= ? AND
					bst1.interval_start < ?
				)
				OR
				(
					bst1.interval_start = (
						SELECT
							MAX(bst2.interval_start)
						FROM
							bucket_storage_tallies bst2
						WHERE
							bst2.project_id = bst1.project_id AND
							bst2.bucket_name = bst1.bucket_name AND
							bst2.interval_start < ?
					)
				)
			)
		ORDER BY bst1.interval_start DESC
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

	// Map to track usages by partner and placement
	// Key format: "partner|placement" where placement is the numeric value
	// If no partner, key is "|placement"
	usages = make(map[string]accounting.ProjectUsage)

	for _, bucket := range buckets {
		key := ""

		if !aggregate {
			valueAttr, err := db.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
				dbx.ValueAttribution_ProjectId(projectID[:]),
				dbx.ValueAttribution_BucketName([]byte(bucket.Name)))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}

			var partner string
			var placement int

			if valueAttr != nil {
				if valueAttr.UserAgent != nil {
					partner, err = tryFindPartnerByUserAgent(valueAttr.UserAgent, partnerNames)
					if err != nil {
						return nil, err
					}
				}

				// Get placement from value attribution
				if valueAttr.Placement != nil {
					placement = *valueAttr.Placement
				}
			}

			// Create a key that combines partner and placement
			key = fmt.Sprintf("%s|%d", partner, placement)
		}

		if _, ok := usages[key]; !ok {
			usages[key] = accounting.ProjectUsage{Since: since, Before: before}
		}
		usage := usages[key]

		storageTalliesRows, err := db.db.QueryContext(ctx, storageQuery, projectID[:], []byte(bucket.Name), since, before, since)
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
				// this first (newest) tally's values are ignored and only used as a
				// fencepost for the next tally we consider (which is older, since we
				// consider these in decreasing order).
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

		totalEgressRow := db.db.QueryRowContext(ctx, totalEgressQuery, projectID[:], []byte(bucket.Name), since, before, int64(pb.PieceAction_GET))

		var egress int64
		if err = totalEgressRow.Scan(&egress); err != nil {
			return nil, err
		}
		usage.Egress += egress

		usages[key] = usage
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
			// Use the standard partner|placement format with default placement 0
			key := fmt.Sprintf("%s|%d", partner, 0)
			usages[key] = accounting.ProjectUsage{}
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
// If withInfo is true, it includes the placement and user agent of the bucket.
func (db *ProjectAccounting) GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time, withInfo bool) (_ []accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	since = timeTruncateDown(since.UTC())
	before = before.UTC()

	buckets, err := db.GetBucketsSinceAndBefore(ctx, projectID, since, before, withInfo)
	if err != nil {
		return nil, err
	}

	var bucketUsageRollups []accounting.BucketUsageRollup
	for _, bucket := range buckets {
		bucketRollup, err := db.getSingleBucketRollup(ctx, projectID, bucket.Name, since, before)
		if err != nil {
			return nil, err
		}

		if bucket.Placement != nil {
			bucketRollup.Placement = *bucket.Placement
		}
		bucketRollup.UserAgent = bucket.UserAgent

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
	case dbutil.Postgres, dbutil.Spanner:
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

// GetSingleBucketTotals retrieves single bucket usage totals for period of time.
func (db *ProjectAccounting) GetSingleBucketTotals(ctx context.Context, projectID uuid.UUID, bucketName string, before time.Time) (usage *accounting.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	bucketQuery := db.db.Rebind(`
		SELECT versioning, placement, object_lock_enabled, created_at, default_retention_mode, default_retention_days, default_retention_years
		FROM bucket_metainfos
		WHERE project_id = ? AND name = ?
	`)
	bucketRow := db.db.QueryRowContext(ctx, bucketQuery, projectID[:], []byte(bucketName))

	var bucketData struct {
		versioning            satbuckets.Versioning
		objectLockEnabled     bool
		placement             storj.PlacementConstraint
		createdAt             time.Time
		defaultRetentionMode  *storj.RetentionMode
		defaultRetentionDays  *int
		defaultRetentionYears *int
	}

	err = bucketRow.Scan(
		&bucketData.versioning,
		&bucketData.placement,
		&bucketData.objectLockEnabled,
		&bucketData.createdAt,
		&bucketData.defaultRetentionMode,
		&bucketData.defaultRetentionDays,
		&bucketData.defaultRetentionYears,
	)
	if err != nil {
		return nil, err
	}

	usage = &accounting.BucketUsage{
		ProjectID:             projectID,
		BucketName:            bucketName,
		Versioning:            bucketData.versioning,
		ObjectLockEnabled:     bucketData.objectLockEnabled,
		DefaultPlacement:      bucketData.placement,
		Since:                 bucketData.createdAt,
		Before:                before,
		DefaultRetentionDays:  bucketData.defaultRetentionDays,
		DefaultRetentionYears: bucketData.defaultRetentionYears,
		CreatedAt:             bucketData.createdAt,
	}
	if bucketData.defaultRetentionMode != nil {
		usage.DefaultRetentionMode = *bucketData.defaultRetentionMode
	}

	rollupsQuery := db.db.Rebind(`SELECT COALESCE(SUM(settled) + SUM(inline), 0)
		FROM bucket_bandwidth_rollups
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ? AND action = ?`)

	// use int64 for compatibility with Spanner
	actionGet := int64(pb.PieceAction_GET)
	rollupRow := db.db.QueryRowContext(ctx, rollupsQuery, projectID[:], []byte(bucketName), bucketData.createdAt, before, actionGet)

	var egress int64
	err = rollupRow.Scan(&egress)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	usage.Egress = memory.Size(egress).GB()

	storageQuery := db.db.Rebind(`SELECT total_bytes, inline, remote, object_count, total_segments_count
		FROM bucket_storage_tallies
		WHERE project_id = ? AND bucket_name = ? AND interval_start >= ? AND interval_start <= ?
		ORDER BY interval_start DESC
		LIMIT 1`)

	storageRow := db.db.QueryRowContext(ctx, storageQuery, projectID[:], []byte(bucketName), bucketData.createdAt, before)

	var (
		tally          accounting.BucketStorageTally
		inline, remote int64
	)
	err = storageRow.Scan(&tally.TotalBytes, &inline, &remote, &tally.ObjectCount, &tally.TotalSegmentCount)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	if tally.TotalBytes == 0 {
		tally.TotalBytes = inline + remote
	}

	usage.Storage = memory.Size(tally.Bytes()).GB()
	usage.SegmentCount = tally.TotalSegmentCount
	usage.ObjectCount = tally.ObjectCount

	return usage, nil
}

// GetBucketTotals retrieves bucket usage totals for period of time.
func (db *ProjectAccounting) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, since, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}
	if cursor.Page == 0 {
		return nil, errs.New("page can not be 0")
	}

	bucketPrefix := []byte(cursor.Search)

	bucketNameRange, incrPrefix, err := db.prefixMatch("name", bucketPrefix)
	if err != nil {
		return nil, err
	}

	emailSearch := "%" + strings.ToLower(cursor.Search) + "%"
	emailExpr := "CASE WHEN pm.member_id IS NOT NULL THEN u.email ELSE '' END"
	whereClause := `
		WHERE bm.project_id = ?
		AND (
			` + bucketNameRange + `
          	OR LOWER(` + emailExpr + `) LIKE ?
        )
    `

	countQuery := db.db.Rebind(`
		SELECT COUNT(*) 
		FROM bucket_metainfos bm
		LEFT JOIN users u
			ON u.id = bm.created_by
		LEFT JOIN project_members pm
			ON pm.project_id = bm.project_id
			AND pm.member_id = bm.created_by
    ` + whereClause)

	countArgs := []interface{}{projectID[:], bucketPrefix}
	if incrPrefix != nil {
		countArgs = append(countArgs, incrPrefix)
	}
	countArgs = append(countArgs, emailSearch)

	page := &accounting.BucketUsagePage{
		Search: cursor.Search,
		Limit:  cursor.Limit,
		Offset: uint64((cursor.Page - 1) * cursor.Limit),
	}

	if err = db.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&page.TotalCount); err != nil {
		return nil, err
	}

	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return nil, errs.New("page is out of range")
	}

	bucketsQuery := db.db.Rebind(`
		SELECT
			bm.name,
			bm.user_agent,
			bm.versioning,
			bm.placement,
			bm.object_lock_enabled,
			bm.default_retention_mode,
			bm.default_retention_days,
			bm.default_retention_years,
			bm.created_at,
			` + emailExpr + ` AS creator_email
		FROM bucket_metainfos bm
		LEFT JOIN users u
			ON u.id = bm.created_by
		LEFT JOIN project_members pm
			ON pm.project_id = bm.project_id
			AND pm.member_id = bm.created_by
    	` + whereClause + `
  		ORDER BY bm.name ASC
		LIMIT ? OFFSET ?
    `)

	pageArgs := []interface{}{projectID[:], bucketPrefix}
	if incrPrefix != nil {
		pageArgs = append(pageArgs, incrPrefix)
	}
	pageArgs = append(pageArgs, emailSearch, page.Limit, page.Offset)

	rows, err := db.db.QueryContext(ctx, bucketsQuery, pageArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var usages []accounting.BucketUsage
	for rows.Next() {
		var (
			bucket                string
			userAgent             []byte
			versioning            satbuckets.Versioning
			objectLockEnabled     bool
			placement             storj.PlacementConstraint
			defaultRetentionMode  *storj.RetentionMode
			defaultRetentionDays  *int
			defaultRetentionYears *int
			createdAt             time.Time
			creatorEmail          string
		)
		err = rows.Scan(
			&bucket,
			&userAgent,
			&versioning,
			&placement,
			&objectLockEnabled,
			&defaultRetentionMode,
			&defaultRetentionDays,
			&defaultRetentionYears,
			&createdAt,
			&creatorEmail,
		)
		if err != nil {
			return nil, err
		}

		usage := accounting.BucketUsage{
			ProjectID:             projectID,
			BucketName:            bucket,
			UserAgent:             userAgent,
			Versioning:            versioning,
			ObjectLockEnabled:     objectLockEnabled,
			DefaultPlacement:      placement,
			DefaultRetentionDays:  defaultRetentionDays,
			DefaultRetentionYears: defaultRetentionYears,
			Since:                 since,
			Before:                before,
			CreatedAt:             createdAt,
			CreatorEmail:          creatorEmail,
		}
		if defaultRetentionMode != nil {
			usage.DefaultRetentionMode = *defaultRetentionMode
		}

		usages = append(usages, usage)
	}
	if err = rows.Err(); err != nil {
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

	for i, usage := range usages {
		// get bucket_bandwidth_rollups
		// use int64 for compatibility with Spanner
		actionGet := int64(pb.PieceAction_GET)
		rollupRow := db.db.QueryRowContext(ctx, rollupsQuery, projectID[:], []byte(usage.BucketName), since, before, actionGet)

		var egress int64
		err = rollupRow.Scan(&egress)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}

		usages[i].Egress = memory.Size(egress).GB()

		storageRow := db.db.QueryRowContext(ctx, storageQuery, projectID[:], []byte(usage.BucketName), since, before)

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
		usages[i].Storage = memory.Size(tally.Bytes()).GB()
		usages[i].SegmentCount = tally.TotalSegmentCount
		usages[i].ObjectCount = tally.ObjectCount
	}

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.BucketUsages = usages
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
		err := db.db.DB.QueryRowContext(ctx, `
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
	case dbutil.Spanner:
		// use INSERT OR UPDATE in case data was archived partially before
		query := `
			INSERT OR UPDATE INTO bucket_bandwidth_rollup_archives(
				bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled
			)
			SELECT bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled
				FROM bucket_bandwidth_rollups WHERE interval_start <= ? LIMIT ?
			THEN RETURN project_id, bucket_name, interval_start, action`

		type rollupToDelete struct {
			ProjectID     []byte
			BucketName    []byte
			IntervalStart time.Time
			Action        int64
		}

		for rowCount := int64(batchSize); rowCount >= int64(batchSize); {
			err := db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
				return withRows(tx.QueryContext(ctx, query, before, batchSize))(func(rows tagsql.Rows) error {
					var toDelete []rollupToDelete
					for rows.Next() {
						var rollup rollupToDelete
						if err := rows.Scan(&rollup.ProjectID, &rollup.BucketName, &rollup.IntervalStart, &rollup.Action); err != nil {
							err = errs.Combine(err, rows.Err(), rows.Close())
							return err
						}
						toDelete = append(toDelete, rollup)
					}

					res, err := tx.ExecContext(ctx, `
						DELETE FROM bucket_bandwidth_rollups
							WHERE STRUCT<ProjectID BYTES, BucketName BYTES, IntervalStart TIMESTAMP, Action INT64>(project_id, bucket_name, interval_start, action) IN UNNEST(?)`,
						toDelete)
					if err != nil {
						return err
					}

					rowCount, err = res.RowsAffected()
					if err != nil {
						return err
					}
					archivedCount += int(rowCount)

					return nil
				})
			})
			if err != nil {
				return 0, Error.Wrap(err)
			}
		}

		return archivedCount, Error.Wrap(err)
	default:
		return 0, nil
	}
}

func (db *ProjectAccounting) archiveRollupsBeforeByAction(ctx context.Context, action int32, before time.Time, batchSize int) (archivedCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		for {
			var rowCount int
			err := db.db.QueryRowContext(ctx, `
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
	case dbutil.Spanner:
		for {
			var rowCount int
			err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
				row := tx.Tx.QueryRowContext(ctx, `
					SELECT count(*) FROM bucket_bandwidth_rollups
					 WHERE action = ? AND interval_start <= ? LIMIT ?
				`, int(action), before, batchSize)
				err = row.Scan(&rowCount)

				if err != nil {
					return Error.Wrap(err)
				}

				archivedCount += rowCount

				_, err = tx.Tx.ExecContext(ctx, `
					INSERT INTO bucket_bandwidth_rollup_archives(bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled)
						SELECT bucket_name, project_id, interval_start, interval_seconds, action, inline, allocated, settled FROM bucket_bandwidth_rollups WHERE action = ? AND
						interval_start <= ? LIMIT ?`,
					int(action), before, batchSize,
				)
				if err != nil {
					return Error.Wrap(err)
				}

				_, err = tx.Tx.ExecContext(ctx, `
					DELETE FROM bucket_bandwidth_rollups WHERE action = ? AND interval_start <= ?
					LIMIT ?`,
					int(action), before, batchSize,
				)

				return Error.Wrap(err)
			})
			if err != nil {
				return archivedCount, err
			}

			if rowCount < batchSize {
				return archivedCount, nil
			}
		}
	default:
		return 0, Error.New("unsupported database: %v", db.db.impl)
	}
}

// GetBucketsSinceAndBefore lists distinct bucket names for a project within a specific timeframe.
// If withInfo is true, it also retrieves bucket information such as placement and user agent.
// Exposed to be tested.
func (db *ProjectAccounting) GetBucketsSinceAndBefore(ctx context.Context, projectID uuid.UUID, since, before time.Time, withInfo bool) (buckets []accounting.BucketInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT DISTINCT bucket_name
		FROM (
			SELECT bst.bucket_name
				FROM bucket_storage_tallies bst
			WHERE bst.project_id = ?
				AND bst.interval_start >= ?
				AND bst.interval_start < ?
		UNION DISTINCT
			SELECT bbr.bucket_name
				FROM bucket_bandwidth_rollups bbr
			WHERE bbr.project_id = ?
				AND bbr.interval_start >= ?
				AND bbr.interval_start < ?
		) combined_buckets`

	if withInfo {
		query = `SELECT DISTINCT bucket_name, placement, user_agent
		FROM (
			SELECT bst.bucket_name, va.placement, va.user_agent
				FROM bucket_storage_tallies bst
			JOIN value_attributions va ON va.bucket_name = bst.bucket_name
			   AND va.project_id = bst.project_id
			WHERE bst.project_id = ?
				AND bst.interval_start >= ?
				AND bst.interval_start < ?
		UNION DISTINCT
			SELECT bbr.bucket_name, va.placement, va.user_agent
				FROM bucket_bandwidth_rollups bbr
			JOIN value_attributions va ON va.bucket_name = bbr.bucket_name
				AND va.project_id = bbr.project_id
			WHERE bbr.project_id = ?
				AND bbr.interval_start >= ?
				AND bbr.interval_start < ?
		) combined_buckets`
	}

	rows, err := db.db.QueryContext(ctx, db.db.Rebind(query), projectID[:], since, before, projectID[:], since, before)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var bucket string
		if withInfo {
			var placement *storj.PlacementConstraint
			var userAgent []byte
			err = rows.Scan(&bucket, &placement, &userAgent)
			if err != nil {
				return nil, err
			}
			buckets = append(buckets, accounting.BucketInfo{
				Name:      bucket,
				Placement: placement,
				UserAgent: userAgent,
			})
			continue
		}

		err = rows.Scan(&bucket)
		if err != nil {
			return nil, errs.Combine(err, rows.Err(), rows.Close())
		}
		buckets = append(buckets, accounting.BucketInfo{Name: bucket})
	}

	err = errs.Combine(rows.Err(), rows.Close())
	if err != nil {
		return nil, err
	}

	return buckets, errs.Combine(err, rows.Err())
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
		ProjectID:        projectID,
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
