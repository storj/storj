// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/s3event"
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
func (t *TiDBAdapter) DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)

	// Limit based on how much TiDB can roughly handle in a single DELETE statement.
	batchSize := min(opts.BatchSize, tidbMaxSegmentBatch)

	deleteBatch := func(ctx context.Context) (deletedObjects, deletedSegments int64, objectsInfo []DeleteObjectsInfo, err error) {
		defer mon.Task()(&ctx)(&err)

		err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
			// reset on retry
			deletedObjects = 0
			deletedSegments = 0
			objectsInfo = nil

			type rec struct {
				streamID       uuid.UUID
				objectKey      ObjectKey
				version        Version
				totalPlainSize int64
			}
			var streamIDs [][]byte
			var keys []rec
			err := dx.WithRows(tx.QueryContext(ctx, `
				SELECT stream_id, object_key, version, status, segment_count, created_at, total_encrypted_size, total_plain_size
				FROM objects
				WHERE (project_id, bucket_name) = (?, ?)
				ORDER BY object_key
				LIMIT ?
				FOR UPDATE
			`, opts.Bucket.ProjectID, opts.Bucket.BucketName, batchSize))(func(rows dx.Rows) error {
				for rows.Next() {
					var streamID uuid.UUID
					var objectKey ObjectKey
					var version Version
					var status int
					var segmentCount int64
					var createdAt time.Time
					var totalEncryptedSize int64
					var totalPlainSize int64
					if err := rows.Scan(&streamID, &objectKey, &version, &status, &segmentCount, &createdAt, &totalEncryptedSize, &totalPlainSize); err != nil {
						return Error.Wrap(err)
					}
					deletedObjects++
					deletedSegments += segmentCount
					streamIDs = append(streamIDs, streamID.Bytes())
					keys = append(keys, rec{
						streamID:       streamID,
						objectKey:      objectKey,
						version:        version,
						totalPlainSize: totalPlainSize,
					})
					if opts.OnObjectsDeleted != nil {
						objectsInfo = append(objectsInfo, DeleteObjectsInfo{
							StreamVersionID:    NewStreamVersionID(version, streamID),
							Status:             ObjectStatus(status),
							CreatedAt:          createdAt,
							TotalEncryptedSize: totalEncryptedSize,
						})
					}
				}
				return nil
			})
			if err != nil {
				return Error.Wrap(err)
			}

			if len(keys) == 0 {
				return nil
			}

			query := `DELETE FROM objects WHERE (project_id, bucket_name, object_key, version) IN (` +
				strings.Repeat("(?,?,?,?),", len(keys)-1) + `(?,?,?,?));` +
				`DELETE FROM segments WHERE stream_id IN (` + tidbPlaceholders(len(streamIDs)) + `)`
			args := make([]any, 0, len(keys)*4+len(streamIDs))
			for _, k := range keys {
				args = append(args, opts.Bucket.ProjectID, opts.Bucket.BucketName, []byte(k.objectKey), int64(k.version))
			}
			for _, sid := range streamIDs {
				args = append(args, sid)
			}
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return Error.Wrap(err)
			}

			if opts.TransmitEvent {
				events := make([]BucketEvent, 0, len(keys))
				for _, k := range keys {
					events = append(events, BucketEvent{
						EventName: s3event.ObjectRemovedDelete.Name(),
						ObjectStream: ObjectStream{
							ProjectID:  opts.Bucket.ProjectID,
							BucketName: opts.Bucket.BucketName,
							ObjectKey:  k.objectKey,
							Version:    k.version,
							StreamID:   k.streamID,
						},
						TotalPlainSize: k.totalPlainSize,
					})
				}
				tidbEnqueueBucketEvent(tx, events...)
			}

			return nil
		})
		if err != nil {
			return 0, 0, nil, err
		}
		return deletedObjects, deletedSegments, objectsInfo, nil
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
