// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
)

const (
	deleteBatchSizeLimit = intLimitRange(50)
)

// DeleteAllBucketObjects contains arguments for deleting a whole bucket.
type DeleteAllBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int

	MaxStaleness   time.Duration
	MaxCommitDelay *time.Duration

	// supported only by Spanner.
	TransmitEvent bool

	// OnObjectsDeleted is called per batch with object info for deleted objects in that batch.
	// When nil, object info is not collected.
	OnObjectsDeleted func([]DeleteObjectsInfo)
}

// DeleteAllBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (db *DB) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjects int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Bucket.Verify(); err != nil {
		return 0, err
	}

	deleteBatchSizeLimit.Ensure(&opts.BatchSize)

	deletedBatchObjectCount, deletedBatchSegmentCount, err := db.ChooseAdapter(opts.Bucket.ProjectID).DeleteAllBucketObjects(ctx, opts)
	mon.Meter("object_delete").Mark64(deletedBatchObjectCount)
	mon.Meter("segment_delete").Mark64(deletedBatchSegmentCount)

	return deletedBatchObjectCount, err
}

const (
	postgresDeleteCTE = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE (project_id, bucket_name) = ($1, $2) AND
				stream_id IN (
					SELECT stream_id FROM objects
					WHERE (project_id, bucket_name) = ($1, $2)
					LIMIT $3
				)
			RETURNING stream_id, version, status, created_at, total_encrypted_size, segment_count
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)`
	cockroachDeleteCTE = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE (project_id, bucket_name) = ($1, $2)
			LIMIT $3
			RETURNING stream_id, version, status, created_at, total_encrypted_size, segment_count
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)`
)

// DeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (p *PostgresAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)
	return tagsqlDeleteAllBucketObjects(ctx, p, opts, postgresDeleteCTE)
}

// DeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (c *CockroachAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)
	return tagsqlDeleteAllBucketObjects(ctx, c, opts, cockroachDeleteCTE)
}

func tagsqlDeleteAllBucketObjects(ctx context.Context, db tagsqlAdapter, opts DeleteAllBucketObjects,
	deleteCTE string,
) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	deleteBatch := func(ctx context.Context) (deletedObjects, deletedSegments int64, objectsInfo []DeleteObjectsInfo, err error) {
		defer mon.Task()(&ctx)(&err)

		if opts.OnObjectsDeleted != nil {
			rows, err := db.UnderlyingDB().QueryContext(ctx,
				deleteCTE+` SELECT stream_id, version, status, created_at, total_encrypted_size, segment_count FROM deleted_objects`,
				opts.Bucket.ProjectID, opts.Bucket.BucketName, opts.BatchSize)
			if err != nil {
				return 0, 0, nil, Error.Wrap(err)
			}
			defer func() { err = errs.Combine(err, rows.Close()) }()

			for rows.Next() {
				var streamID uuid.UUID
				var version Version
				var status int
				var createdAt time.Time
				var totalEncryptedSize int64
				var segmentCount int64
				if err := rows.Scan(&streamID, &version, &status, &createdAt, &totalEncryptedSize, &segmentCount); err != nil {
					return 0, 0, nil, Error.Wrap(err)
				}

				deletedObjects++
				deletedSegments += segmentCount

				objectsInfo = append(objectsInfo, DeleteObjectsInfo{
					StreamVersionID:    NewStreamVersionID(version, streamID),
					Status:             ObjectStatus(status),
					CreatedAt:          createdAt,
					TotalEncryptedSize: totalEncryptedSize,
				})
			}
			return deletedObjects, deletedSegments, objectsInfo, Error.Wrap(rows.Err())
		}

		err = db.UnderlyingDB().QueryRowContext(ctx,
			deleteCTE+` SELECT COUNT(1), COALESCE(SUM(segment_count), 0) FROM deleted_objects`,
			opts.Bucket.ProjectID, opts.Bucket.BucketName, opts.BatchSize).Scan(&deletedObjects, &deletedSegments)
		return deletedObjects, deletedSegments, nil, Error.Wrap(err)
	}

	for {
		deletedObjects, deletedSegments, batchObjectsInfo, err := deleteBatch(ctx)
		if err != nil {
			return totalDeletedObjects, totalDeletedSegments, err
		}

		totalDeletedObjects += deletedObjects
		totalDeletedSegments += deletedSegments

		if opts.OnObjectsDeleted != nil && len(batchObjectsInfo) > 0 {
			opts.OnObjectsDeleted(batchObjectsInfo)
		}

		if deletedObjects == 0 {
			return totalDeletedObjects, totalDeletedSegments, nil
		}
	}
}

// DeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (s *SpannerAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)

	deleteBatch := func(ctx context.Context, cursor ObjectKey) (lastDeletedObject ObjectKey, deletedObjects, deletedSegments int64, objectsInfo []DeleteObjectsInfo, err error) {
		defer mon.Task()(&ctx)(&err)

		// TODO(spanner): see if it would be better to avoid batching altogether here.
		_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
			// Reset counters in case the transaction is retried.
			lastDeletedObject = ObjectKey("")
			deletedObjects = 0
			deletedSegments = 0
			deleteSegments := []*spanner.Mutation{}

			// Include created_at and total_encrypted_size when OnObjectsDeleted callback is set.
			query := `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name) = (@project_id, @bucket_name) AND
					stream_id IN (
						SELECT stream_id FROM objects
						WHERE (project_id, bucket_name) = (@project_id, @bucket_name) AND @cursor <= object_key
						ORDER BY project_id, bucket_name, object_key
						LIMIT @delete_limit
					)
				THEN RETURN object_key, version, status, stream_id, segment_count`
			if opts.OnObjectsDeleted != nil {
				query += `, created_at, total_encrypted_size`
			}

			iter := tx.QueryWithOptions(ctx, spanner.Statement{
				SQL: query,
				Params: map[string]any{
					"project_id":   opts.Bucket.ProjectID,
					"bucket_name":  opts.Bucket.BucketName,
					"delete_limit": opts.BatchSize,
					"cursor":       cursor,
				},
			}, spanner.QueryOptions{RequestTag: "delete-all-bucket-objects"})
			err := iter.Do(func(row *spanner.Row) error {
				var objectKey ObjectKey
				var version Version
				var status ObjectStatus
				var streamID []byte
				var segmentCount int64
				var createdAt time.Time
				var totalEncryptedSize int64

				if opts.OnObjectsDeleted != nil {
					if err := row.Columns(&objectKey, &version, &status, &streamID, &segmentCount, &createdAt, &totalEncryptedSize); err != nil {
						return Error.Wrap(err)
					}
				} else {
					if err := row.Columns(&objectKey, &version, &status, &streamID, &segmentCount); err != nil {
						return Error.Wrap(err)
					}
				}

				if len(streamID) != len(uuid.UUID{}) {
					return Error.New("invalid stream id for object %q version %v", objectKey, version)
				}

				lastDeletedObject = objectKey
				deletedObjects++
				// Note: this miscounts deleted segments for pending objects,
				// because the objects table does not contain up to date segment_count for them.
				deletedSegments += segmentCount

				if opts.OnObjectsDeleted != nil {
					objectsInfo = append(objectsInfo, DeleteObjectsInfo{
						StreamVersionID:    NewStreamVersionID(version, uuid.UUID(streamID)),
						Status:             status,
						CreatedAt:          createdAt,
						TotalEncryptedSize: totalEncryptedSize,
					})
				}

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
				return Error.New("delete objects query error: %w", err)
			}
			if len(deleteSegments) == 0 {
				return nil
			}

			err = tx.BufferWrite(deleteSegments)
			if err != nil {
				return Error.New("delete segments query error: %w", err)
			}
			return nil
		}, spanner.TransactionOptions{
			CommitPriority: spannerpb.RequestOptions_PRIORITY_MEDIUM,
			CommitOptions: spanner.CommitOptions{
				MaxCommitDelay: opts.MaxCommitDelay,
			},
			TransactionTag:              "delete-all-bucket-objects",
			ExcludeTxnFromChangeStreams: !opts.TransmitEvent,
		})
		if err != nil {
			return lastDeletedObject, 0, 0, nil, Error.Wrap(err)
		}
		return lastDeletedObject, deletedObjects, deletedSegments, objectsInfo, nil
	}

	// We query the first object to be deleted to account for a scenario where a bucket has been already partially
	// been deleted and contains a lot of deleted rows, potentially timing out the following delete queries.
	cursor, err := spannerutil.CollectRow(s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT object_key
			FROM objects
			WHERE (project_id, bucket_name) = (@project_id, @bucket_name)
			ORDER BY project_id, bucket_name, object_key
			LIMIT 1
		`,
		Params: map[string]any{
			"project_id":  opts.Bucket.ProjectID,
			"bucket_name": opts.Bucket.BucketName,
		},
	}, spanner.QueryOptions{RequestTag: "delete-all-bucket-objects-prequery"}), func(row *spanner.Row, item *ObjectKey) error {
		return row.Columns(item)
	})
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return 0, 0, nil
		}
		return 0, 0, Error.Wrap(err)
	}

	for {
		lastDeletedObject, deletedObjects, deletedSegments, batchObjectsInfo, err := deleteBatch(ctx, cursor)
		if err != nil {
			return totalDeletedObjects, totalDeletedSegments, err
		}
		cursor = lastDeletedObject

		totalDeletedObjects += deletedObjects
		totalDeletedSegments += deletedSegments

		if opts.OnObjectsDeleted != nil && len(batchObjectsInfo) > 0 {
			opts.OnObjectsDeleted(batchObjectsInfo)
		}

		if deletedObjects == 0 {
			return totalDeletedObjects, totalDeletedSegments, nil
		}
	}
}
