// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"golang.org/x/exp/slices"

	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// BucketTally contains information about aggregate data stored in a bucket.
type BucketTally struct {
	BucketLocation

	ObjectCount        int64
	PendingObjectCount int64

	TotalSegments int64
	TotalBytes    int64

	MetadataSize int64
}

// CollectBucketTallies contains arguments necessary for looping through objects in metabase.
type CollectBucketTallies struct {
	From               BucketLocation
	To                 BucketLocation
	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
	Now                time.Time
}

// Verify verifies CollectBucketTallies request fields.
func (opts *CollectBucketTallies) Verify() error {
	if opts.To.ProjectID.Less(opts.From.ProjectID) {
		return ErrInvalidRequest.New("project ID To is before project ID From")
	}
	if opts.To.ProjectID == opts.From.ProjectID && opts.To.BucketName < opts.From.BucketName {
		return ErrInvalidRequest.New("bucket name To is before bucket name From")
	}
	return nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (db *DB) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return []BucketTally{}, err
	}

	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	for _, adapter := range db.adapters {
		adapterResult, err := adapter.CollectBucketTallies(ctx, opts)
		if err != nil {
			return nil, err
		}
		result = append(result, adapterResult...)
	}

	// only a merge sort should be strictly required here, but this is much easier to implement for now
	slices.SortFunc(result, func(a, b BucketTally) int {
		cmp := a.ProjectID.Compare(b.ProjectID)
		if cmp != 0 {
			return cmp
		}
		return strings.Compare(a.BucketName, b.BucketName)
	})

	return result, nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (p *PostgresAdapter) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	err = withRows(p.db.QueryContext(ctx, `
			SELECT
				project_id, bucket_name,
				SUM(total_encrypted_size), SUM(segment_count), COALESCE(SUM(length(encrypted_metadata)), 0),
				count(*), count(*) FILTER (WHERE status = `+statusPending+`)
			FROM objects
			`+LimitedAsOfSystemTime(p.impl, time.Now(), opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
			WHERE (project_id, bucket_name) BETWEEN ($1, $2) AND ($3, $4) AND
			(expires_at IS NULL OR expires_at > $5)
			GROUP BY (project_id, bucket_name)
			ORDER BY (project_id, bucket_name) ASC
		`, opts.From.ProjectID, []byte(opts.From.BucketName), opts.To.ProjectID, []byte(opts.To.BucketName), opts.Now))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var bucketTally BucketTally

			if err = rows.Scan(
				&bucketTally.ProjectID, &bucketTally.BucketName,
				&bucketTally.TotalBytes, &bucketTally.TotalSegments,
				&bucketTally.MetadataSize, &bucketTally.ObjectCount,
				&bucketTally.PendingObjectCount,
			); err != nil {
				return Error.New("unable to query bucket tally: %w", err)
			}

			result = append(result, bucketTally)
		}

		return nil
	})
	if err != nil {
		return []BucketTally{}, err
	}

	return result, nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (s *SpannerAdapter) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	return spannerutil.CollectRows(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			WITH counts AS (
				SELECT project_id, bucket_name, segment_count, total_encrypted_size, length(encrypted_metadata) AS encrypted_bytes, status
				FROM objects
				WHERE
					` + TupleGreaterThanSQL([]string{"project_id", "bucket_name"}, []string{"@from_project_id", "@from_bucket_name"}, true) + `
					AND ` + TupleGreaterThanSQL([]string{"@to_project_id", "@to_bucket_name"}, []string{"project_id", "bucket_name"}, true) + `
					AND (expires_at IS NULL OR expires_at > @when)
			)
			SELECT
				project_id, bucket_name,
				SUM(total_encrypted_size), SUM(segment_count), COALESCE(SUM(encrypted_bytes), 0),
				count(*), (
					SELECT count(*) FROM counts c2 WHERE status = ` + statusPending + `
						AND c2.project_id = c.project_id AND c2.bucket_name = c.bucket_name
				)
			FROM counts c
			GROUP BY project_id, bucket_name
			ORDER BY project_id ASC, bucket_name ASC
		`,
		Params: map[string]any{
			"from_project_id":  opts.From.ProjectID,
			"from_bucket_name": opts.From.BucketName,
			"to_project_id":    opts.To.ProjectID,
			"to_bucket_name":   opts.To.BucketName,
			"when":             opts.Now,
		},
	}), func(row *spanner.Row, bucketTally *BucketTally) error {
		return row.Columns(
			&bucketTally.ProjectID, &bucketTally.BucketName,
			&bucketTally.TotalBytes, &bucketTally.TotalSegments,
			&bucketTally.MetadataSize, &bucketTally.ObjectCount,
			&bucketTally.PendingObjectCount,
		)
	})
}
