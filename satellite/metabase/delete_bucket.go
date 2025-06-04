// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
)

const (
	deleteBatchSizeLimit = intLimitRange(50)
)

// DeleteAllBucketObjects contains arguments for deleting a whole bucket.
type DeleteAllBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int
}

// DeleteAllBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (db *DB) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjectCount int64, err error) {
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
		deletedBatchObjectCount, deletedBatchSegmentCount, err = db.ChooseAdapter(opts.Bucket.ProjectID).DeleteAllBucketObjects(ctx, opts)

		mon.Meter("object_delete").Mark64(deletedBatchObjectCount)
		mon.Meter("segment_delete").Mark64(deletedBatchSegmentCount)

		deletedObjectCount += deletedBatchObjectCount
		if err != nil {
			return deletedObjectCount, err
		}
	}

	return deletedObjectCount, nil
}

// DeleteAllBucketObjects deletes objects in the specified bucket up to opts.BatchSize number of
// objects.
func (p *PostgresAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE (project_id, bucket_name) = ($1, $2) AND
				stream_id IN (
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

	err = p.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, opts.Bucket.BucketName, opts.BatchSize).Scan(&deletedObjectCount, &deletedSegmentCount)
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}
	return deletedObjectCount, deletedSegmentCount, nil
}

// DeleteAllBucketObjects deletes objects in the specified bucket up to opts.BatchSize number of
// objects.
func (c *CockroachAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
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

	err = c.db.QueryRowContext(ctx, query, opts.Bucket.ProjectID, opts.Bucket.BucketName, opts.BatchSize).Scan(&deletedObjectCount, &deletedSegmentCount)
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}
	return deletedObjectCount, deletedSegmentCount, nil
}

// DeleteAllBucketObjects deletes objects in the specified bucket up to opts.BatchSize number of
// objects.
func (s *SpannerAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var maxCommitDelay = 50 * time.Millisecond

	// TODO(spanner): see if it would be better to avoid batching altogether here.
	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		deleteSegments := []*spanner.Mutation{}

		iter := tx.Query(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name) = (@project_id, @bucket_name) AND
					stream_id IN (
						SELECT stream_id FROM objects
						WHERE (project_id, bucket_name) = (@project_id, @bucket_name)
						ORDER BY project_id, bucket_name
						LIMIT @delete_limit
					)
				THEN RETURN status, stream_id, segment_count
			`,
			Params: map[string]interface{}{
				"project_id":   opts.Bucket.ProjectID,
				"bucket_name":  opts.Bucket.BucketName,
				"delete_limit": opts.BatchSize,
			},
		})
		err := iter.Do(func(row *spanner.Row) error {
			var status ObjectStatus
			var streamID []byte
			var segmentCount int64
			if err := row.Columns(&status, &streamID, &segmentCount); err != nil {
				return Error.Wrap(err)
			}

			deletedObjectCount++
			// Note: this miscounts deleted segments for pending objects,
			// because the objects table does not contain up to date segment_count for them.
			deletedSegmentCount += segmentCount

			if segmentCount > 0 || status.IsPending() {
				deleteSegments = append(deleteSegments,
					spanner.Delete("segments", spanner.KeyRange{
						Start: spanner.Key{streamID},
						End:   spanner.Key{streamID},
						Kind:  spanner.ClosedClosed,
					}))
			}
			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}
		if len(deleteSegments) == 0 {
			return nil
		}

		err = tx.BufferWrite(deleteSegments)
		if err != nil {
			return err
		}
		return nil
	}, spanner.TransactionOptions{
		CommitPriority: spannerpb.RequestOptions_PRIORITY_MEDIUM,
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: &maxCommitDelay,
		},
		TransactionTag: "delete-all-bucket-objects",
	})
	if err != nil {
		return 0, 0, err
	}

	return deletedObjectCount, deletedSegmentCount, nil
}
