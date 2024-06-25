// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"cloud.google.com/go/spanner"

	"storj.io/storj/shared/dbutil/spannerutil"
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

	deletedBatchObjectCount := int64(opts.BatchSize)
	for deletedBatchObjectCount > 0 {
		if err := ctx.Err(); err != nil {
			return deletedObjectCount, err
		}

		var deletedBatchSegmentCount int64
		deletedBatchObjectCount, deletedBatchSegmentCount, err = db.ChooseAdapter(opts.Bucket.ProjectID).DeleteBucketObjects(ctx, opts)

		mon.Meter("object_delete").Mark64(deletedBatchObjectCount)
		mon.Meter("segment_delete").Mark64(deletedBatchSegmentCount)

		deletedObjectCount += deletedBatchObjectCount
		if err != nil {
			return deletedObjectCount, err
		}
	}

	return deletedObjectCount, nil
}

// DeleteBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (p *PostgresAdapter) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
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

	err = p.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize).Scan(&deletedObjectCount, &deletedSegmentCount)
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}
	return deletedObjectCount, deletedSegmentCount, nil
}

// DeleteBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (c *CockroachAdapter) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
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

	err = c.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize).Scan(&deletedObjectCount, &deletedSegmentCount)
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}
	return deletedObjectCount, deletedSegmentCount, nil
}

// DeleteBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (s *SpannerAdapter) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(spanner): see if it would be better to avoid batching altogether here.
	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		streamIDs, err := spannerutil.CollectRows(tx.Query(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE stream_id IN (
					SELECT stream_id FROM objects
					WHERE project_id = @project_id AND bucket_name = @bucket_name
					ORDER BY project_id, bucket_name
					LIMIT @delete_limit
				)
				THEN RETURN stream_id
			`,
			Params: map[string]interface{}{
				"project_id":   opts.Bucket.ProjectID,
				"bucket_name":  opts.Bucket.BucketName,
				"delete_limit": opts.BatchSize,
			},
		}), func(row *spanner.Row, streamID *[]byte) error {
			return row.Columns(streamID)
		})
		if err != nil {
			return Error.Wrap(err)
		}
		deletedObjectCount = int64(len(streamIDs))
		if len(streamIDs) == 0 {
			return nil
		}

		deletedSegmentCount, err = tx.Update(ctx, spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE stream_id IN UNNEST(@stream_ids)
			`,
			Params: map[string]interface{}{
				"stream_ids": streamIDs,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return deletedObjectCount, deletedSegmentCount, nil
}
