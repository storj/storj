// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/tagsql"
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

	err = withRows(db.db.QueryContext(ctx, `
			SELECT
				project_id, bucket_name,
				SUM(total_encrypted_size), SUM(segment_count), COALESCE(SUM(length(encrypted_metadata)), 0),
				count(*), count(*) FILTER (WHERE status = `+statusPending+`)
			FROM objects
			`+db.asOfTime(opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
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
