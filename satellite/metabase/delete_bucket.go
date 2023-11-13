// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/private/dbutil"
)

const (
	deleteBatchSizeLimit = intLimitRange(50)
)

// DeleteBucketObjects contains arguments for deleting a whole bucket.
type DeleteBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int
}

// DeleteBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (db *DB) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Bucket.Verify(); err != nil {
		return 0, err
	}

	deleteBatchSizeLimit.Ensure(&opts.BatchSize)

	// TODO we may think about doing pending and committed objects in parallel
	deletedBatchCount := int64(opts.BatchSize)
	for deletedBatchCount > 0 {
		if err := ctx.Err(); err != nil {
			return deletedObjectCount, err
		}

		deletedBatchCount, err = db.deleteBucketObjects(ctx, opts)
		deletedObjectCount += deletedBatchCount

		if err != nil {
			return deletedObjectCount, err
		}
	}

	deletedBatchCount = int64(opts.BatchSize)
	for deletedBatchCount > 0 {
		if err := ctx.Err(); err != nil {
			return deletedObjectCount, err
		}

		deletedBatchCount, err = db.deleteBucketPendingObjects(ctx, opts)
		deletedObjectCount += deletedBatchCount

		if err != nil {
			return deletedObjectCount, err
		}
	}

	return deletedObjectCount, nil
}

func (db *DB) deleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string

	switch db.impl {
	case dbutil.Cockroach:
		query = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE (project_id, bucket_name) = ($1, $2)
			LIMIT $3
			RETURNING objects.stream_id, objects.segment_count
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT COUNT(1), COALESCE(SUM(segment_count), 0) FROM deleted_objects
	`
	case dbutil.Postgres:
		query = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE stream_id IN (
				SELECT stream_id FROM objects
				WHERE (project_id, bucket_name) = ($1, $2)
				LIMIT $3
			)
			RETURNING objects.stream_id, objects.segment_count
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT COUNT(1), COALESCE(SUM(segment_count), 0) FROM deleted_objects
	`
	default:
		return 0, Error.New("unhandled database: %v", db.impl)
	}

	var deletedSegmentCount int64
	err = db.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize).Scan(&deletedObjectCount, &deletedSegmentCount)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	mon.Meter("object_delete").Mark64(deletedObjectCount)
	mon.Meter("segment_delete").Mark64(deletedSegmentCount)

	return deletedObjectCount, nil
}

func (db *DB) deleteBucketPendingObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string

	// TODO handle number of deleted segments
	switch db.impl {
	case dbutil.Cockroach:
		query = `
		WITH deleted_objects AS (
			DELETE FROM pending_objects
			WHERE (project_id, bucket_name) = ($1, $2)
			LIMIT $3
			RETURNING pending_objects.stream_id
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT COUNT(1) FROM deleted_objects
	`
	case dbutil.Postgres:
		query = `
		WITH deleted_objects AS (
			DELETE FROM pending_objects
			WHERE stream_id IN (
				SELECT stream_id FROM pending_objects
				WHERE (project_id, bucket_name) = ($1, $2)
				LIMIT $3
			)
			RETURNING pending_objects.stream_id
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT COUNT(1) FROM deleted_objects
	`
	default:
		return 0, Error.New("unhandled database: %v", db.impl)
	}

	err = db.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize).Scan(&deletedObjectCount)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	mon.Meter("object_delete").Mark64(deletedObjectCount)

	return deletedObjectCount, nil
}
