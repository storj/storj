// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// ListVerifyLimit is the maximum number of items the client can request for listing.
const ListVerifyLimit = intLimitRange(100000)

// ListVerifyBucketList represents a list of buckets.
type ListVerifyBucketList struct {
	Buckets []BucketLocation
}

// Add adds a (projectID, bucketName) to the list of buckets to be checked.
func (list *ListVerifyBucketList) Add(projectID uuid.UUID, bucketName BucketName) {
	list.Buckets = append(list.Buckets, BucketLocation{
		ProjectID:  projectID,
		BucketName: bucketName,
	})
}

// ListBucketsStreamIDsResult is the result of listing segments of a list of buckets.
type ListBucketsStreamIDsResult struct {
	StreamIDs  []uuid.UUID
	Counts     []int
	LastBucket BucketLocation
}

func (list *ListBucketsStreamIDsResult) addStreamID(streamID uuid.UUID, count int) {
	list.StreamIDs = append(list.StreamIDs, streamID)
	list.Counts = append(list.Counts, count)
}

// ListBucketsStreamIDs contains arguments necessary for listing stream segments from buckets.
type ListBucketsStreamIDs struct {
	BucketList     ListVerifyBucketList
	CursorBucket   BucketLocation
	CursorStreamID uuid.UUID
	Limit          int

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
}

// ListBucketsStreamIDs lists the streamIDs of a list of buckets.
func (db *DB) ListBucketsStreamIDs(ctx context.Context, opts ListBucketsStreamIDs) (ListBucketsStreamIDsResult, error) {

	if opts.Limit <= 0 {
		return ListBucketsStreamIDsResult{}, ErrInvalidRequest.New("invalid limit: %d", opts.Limit)
	}
	ListVerifyLimit.Ensure(&opts.Limit)

	result := ListBucketsStreamIDsResult{}

	bucketNamesBytes := [][]byte{}
	projectIDs := []uuid.UUID{}
	for _, bucket := range opts.BucketList.Buckets {
		bucketNamesBytes = append(bucketNamesBytes, []byte(bucket.BucketName))
		projectIDs = append(projectIDs, bucket.ProjectID)
	}

	for _, adapter := range db.adapters {
		adapterResult, err := adapter.ListBucketsStreamIDs(ctx, opts, bucketNamesBytes, projectIDs)
		if err != nil {
			return ListBucketsStreamIDsResult{}, Error.Wrap(err)
		}
		result.StreamIDs = append(result.StreamIDs, adapterResult.StreamIDs...)
		result.Counts = append(result.Counts, adapterResult.Counts...)
		if adapterResult.LastBucket.Compare(result.LastBucket) > 0 {
			result.LastBucket = adapterResult.LastBucket
		}
	}

	return result, nil
}

// ListBucketsStreamIDs lists the streamIDs of a list of buckets.
func (p *PostgresAdapter) ListBucketsStreamIDs(ctx context.Context, opts ListBucketsStreamIDs, bucketNamesBytes [][]byte, projectIDs []uuid.UUID) (result ListBucketsStreamIDsResult, err error) {
	// get the list of stream_ids and segment counts from the objects table
	err = withRows(p.db.QueryContext(ctx, `
		SELECT DISTINCT project_id, bucket_name, stream_id, segment_count
		FROM objects
		`+LimitedAsOfSystemTime(p.impl, time.Now(), opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
		WHERE
			 (project_id, bucket_name, stream_id) > ($4::BYTEA, $5::BYTEA, $6::BYTEA) AND
		(project_id, bucket_name) IN (SELECT UNNEST($1::BYTEA[]),UNNEST($2::BYTEA[]))
		ORDER BY project_id, bucket_name, stream_id ASC
		LIMIT $3
	`, pgutil.UUIDArray(projectIDs), pgutil.ByteaArray(bucketNamesBytes),
		opts.Limit,
		opts.CursorBucket.ProjectID, opts.CursorBucket.BucketName, opts.CursorStreamID,
	))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var streamID uuid.UUID
			var count int
			err := rows.Scan(
				&result.LastBucket.ProjectID,
				&result.LastBucket.BucketName,
				&streamID,
				&count,
			)
			if err != nil {
				return Error.Wrap(err)
			}
			result.addStreamID(streamID, count)
		}
		return nil
	})
	if err != nil {
		return ListBucketsStreamIDsResult{}, err
	}
	return result, nil
}

// ListBucketsStreamIDs lists the streamIDs of a list of buckets.
func (s *SpannerAdapter) ListBucketsStreamIDs(ctx context.Context, opts ListBucketsStreamIDs, bucketNamesBytes [][]byte, projectIDs []uuid.UUID) (result ListBucketsStreamIDsResult, err error) {
	projectsAndBuckets := make([]struct {
		ProjectID  uuid.UUID
		BucketName BucketName
	}, len(projectIDs))
	for i, projectID := range projectIDs {
		projectsAndBuckets[i].ProjectID = projectID
		projectsAndBuckets[i].BucketName = BucketName(bucketNamesBytes[i])
	}

	tuple, err := spannerutil.TupleGreaterThanSQL([]string{"project_id", "bucket_name", "stream_id"}, []string{"@cursor_project_id", "@cursor_bucket_name", "@cursor_stream_id"}, false)
	if err != nil {
		return result, Error.Wrap(err)
	}

	// get the list of stream_ids and segment counts from the objects table
	// TODO(spanner): check if there is a performance penalty to using a STRUCT in this way.
	err = s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT DISTINCT project_id, bucket_name, stream_id, segment_count
			FROM objects
			WHERE ` + tuple + `
				AND STRUCT<ProjectID BYTES, BucketName STRING>(project_id, bucket_name) IN UNNEST(@projects_and_buckets)
			ORDER BY project_id, bucket_name, stream_id
			LIMIT @limit
		`,
		Params: map[string]any{
			"projects_and_buckets": projectsAndBuckets,
			"cursor_project_id":    opts.CursorBucket.ProjectID,
			"cursor_bucket_name":   opts.CursorBucket.BucketName,
			"cursor_stream_id":     opts.CursorStreamID,
			"limit":                int64(opts.Limit),
		},
	}).Do(func(row *spanner.Row) error {
		var streamID uuid.UUID
		var count int64
		err := row.Columns(
			&result.LastBucket.ProjectID,
			&result.LastBucket.BucketName,
			&streamID,
			&count,
		)
		if err != nil {
			return Error.Wrap(err)
		}
		result.addStreamID(streamID, int(count))
		return nil
	})
	if err != nil {
		return ListBucketsStreamIDsResult{}, err
	}
	return result, nil
}
