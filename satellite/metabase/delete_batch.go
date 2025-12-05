// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/tagsql"
)

// DeleteObjectsAndSegmentsNoVerify deletes expired objects and associated segments.
func (p *PostgresAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return 0, 0, nil
	}

	projectIDs := make([]uuid.UUID, len(objects))
	bucketNames := make([][]byte, len(objects))
	objectKeys := make([][]byte, len(objects))
	versions := make([]int64, len(objects))
	streamIDs := make([]uuid.UUID, len(objects))

	for i, obj := range objects {
		projectIDs[i] = obj.ProjectID
		bucketNames[i] = []byte(obj.BucketName)
		objectKeys[i] = []byte(obj.ObjectKey)
		versions[i] = int64(obj.Version)
		streamIDs[i] = obj.StreamID
	}

	result, err := p.db.ExecContext(ctx, `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE (project_id, bucket_name, object_key, version, stream_id) IN
			(SELECT UNNEST($1::BYTEA[]), UNNEST($2::BYTEA[]), UNNEST($3::BYTEA[]), UNNEST($4::INT8[]), UNNEST($5::BYTEA[]))
			RETURNING stream_id
		)
		DELETE FROM segments
		WHERE segments.stream_id IN (SELECT stream_id FROM deleted_objects)
	`, pgutil.UUIDArray(projectIDs), pgutil.ByteaArray(bucketNames), pgutil.ByteaArray(objectKeys),
		pgutil.Int8Array(versions), pgutil.UUIDArray(streamIDs))
	if err != nil {
		return 0, 0, Error.New("unable to delete expired objects: %w", err)
	}

	affectedSegmentCount, err := result.RowsAffected()
	if err != nil {
		return 0, 0, Error.New("unable to delete expired objects: %w", err)
	}

	if affectedSegmentCount > 0 {
		// Note, this slightly miscounts objects without any segments
		// there doesn't seem to be a simple work around for this.
		// Luckily, this is used only for metrics, where it's not a
		// significant problem to slightly miscount.
		objectsDeleted = int64(len(objects))
		segmentsDeleted += affectedSegmentCount
	}

	return objectsDeleted, segmentsDeleted, nil
}

// DeleteObjectsAndSegmentsNoVerify deletes expired objects and associated segments.
//
// The implementation does not do extra verification whether the stream id-s belong or belonged to the objects.
// So, if the callers supplies objects with incorrect StreamID-s it may end up deleting unrelated segments.
func (s *SpannerAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return 0, 0, nil
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		var streamIDs [][]byte
		for _, obj := range objects {
			streamIDs = append(streamIDs, obj.StreamID.Bytes())
		}

		deletedCounts, err := tx.BatchUpdateWithOptions(ctx, []spanner.Statement{
			{
				SQL: `
					DELETE FROM objects
					WHERE STRUCT<ProjectID BYTES, BucketName STRING, ObjectKey BYTES, Version INT64, StreamID BYTES>(project_id, bucket_name, object_key, version, stream_id) IN UNNEST(@objects)
				`,
				Params: map[string]any{
					"objects": objects,
				},
			},
			{
				SQL: `
					DELETE FROM segments
					WHERE stream_id IN UNNEST(@stream_ids)
				`,
				Params: map[string]any{
					"stream_ids": streamIDs,
				},
			},
		}, spanner.QueryOptions{
			Priority: spannerpb.RequestOptions_PRIORITY_LOW,
		})
		if err != nil {
			return err
		}

		objectsDeleted = deletedCounts[0]
		segmentsDeleted = deletedCounts[1]
		return nil
	}, spanner.TransactionOptions{
		CommitPriority:              spannerpb.RequestOptions_PRIORITY_LOW,
		TransactionTag:              "delete-objects-no-verify",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		return 0, 0, Error.New("unable to delete expired objects: %w", err)
	}
	return objectsDeleted, segmentsDeleted, nil
}

// DeleteInactiveObjectsAndSegments deletes inactive objects and associated segments.
func (p *PostgresAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, objects []ObjectStream, opts DeleteZombieObjects) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return 0, 0, nil
	}

	err = pgxutil.Conn(ctx, p.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch
		for _, obj := range objects {
			batch.Queue(`
				WITH check_segments AS (
					SELECT 1 FROM segments
					WHERE stream_id = $5::BYTEA AND created_at > $6
				), deleted_objects AS (
					DELETE FROM objects
					WHERE
						(project_id, bucket_name, object_key, version) = ($1::BYTEA, $2::BYTEA, $3::BYTEA, $4) AND
						NOT EXISTS (SELECT 1 FROM check_segments)
					RETURNING stream_id
				)
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT stream_id FROM deleted_objects)
			`, obj.ProjectID, obj.BucketName, []byte(obj.ObjectKey), obj.Version, obj.StreamID, opts.InactiveDeadline)
		}

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		// TODO calculate deleted objects
		var errList errs.Group
		for i := 0; i < batch.Len(); i++ {
			result, err := results.Exec()
			errList.Add(err)

			if err == nil {
				segmentsDeleted += result.RowsAffected()
			}
		}

		return errList.Err()
	})
	if err != nil {
		return objectsDeleted, segmentsDeleted, Error.New("unable to delete zombie objects: %w", err)
	}
	return objectsDeleted, segmentsDeleted, nil
}

// DeleteInactiveObjectsAndSegments deletes inactive objects and associated segments.
func (s *SpannerAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, objects []ObjectStream, opts DeleteZombieObjects) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return 0, 0, nil
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// can't use Mutations here, since we only want to delete objects by the specified keys
		// if and only if the stream_id matches and no associated segments were uploaded after
		// opts.InactiveDeadline.
		var statements []spanner.Statement
		for _, obj := range objects {
			obj := obj
			statements = append(statements, spanner.Statement{
				SQL: `
					DELETE FROM objects
					WHERE
						(project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id)
						AND NOT EXISTS (
							SELECT 1 FROM segments
							WHERE
								segments.stream_id = objects.stream_id
								AND segments.created_at > @inactive_deadline
						)
				`,
				Params: map[string]interface{}{
					"project_id":        obj.ProjectID,
					"bucket_name":       obj.BucketName,
					"object_key":        obj.ObjectKey,
					"version":           obj.Version,
					"stream_id":         obj.StreamID,
					"inactive_deadline": opts.InactiveDeadline,
				},
			})
		}

		numDeleteds, err := tx.BatchUpdateWithOptions(ctx, statements, spanner.QueryOptions{
			Priority: spannerpb.RequestOptions_PRIORITY_LOW,
		})
		if err != nil {
			return Error.Wrap(err)
		}

		streamIDs := make([][]byte, 0, len(objects))
		for i, numDeleted := range numDeleteds {
			if numDeleted > 0 {
				streamIDs = append(streamIDs, objects[i].StreamID.Bytes())
			}
			objectsDeleted += numDeleted
		}

		numSegments, err := tx.UpdateWithOptions(ctx, spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE stream_id IN UNNEST(@stream_ids)
			`,
			Params: map[string]interface{}{
				"stream_ids": streamIDs,
			},
		}, spanner.QueryOptions{
			Priority: spannerpb.RequestOptions_PRIORITY_LOW,
		})
		if err != nil {
			return Error.Wrap(err)
		}
		segmentsDeleted += numSegments
		return nil
	}, spanner.TransactionOptions{
		CommitPriority:              spannerpb.RequestOptions_PRIORITY_LOW,
		TransactionTag:              "delete-inactive-objects",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		return objectsDeleted, segmentsDeleted, Error.New("unable to delete zombie objects: %w", err)
	}
	return objectsDeleted, segmentsDeleted, nil
}

// processObjectStreamBatches scans and processes object streams in batches of batchSize based on the query.
func (p *PostgresAdapter) processObjectStreamBatches(ctx context.Context, asOfSystemInterval time.Duration, batchSize int, stmt postgresStatement, process func(context.Context, []ObjectStream) error) (err error) {
	return Error.Wrap(withRows(
		p.db.QueryContext(ctx, stmt.SQL, stmt.Params...),
	)(func(rows tagsql.Rows) error {
		batch := make([]ObjectStream, 0, batchSize)
		for rows.Next() {
			var stream ObjectStream
			if err := rows.Scan(&stream.ProjectID, &stream.BucketName, &stream.ObjectKey, &stream.Version, &stream.StreamID); err != nil {
				return Error.Wrap(err)
			}
			batch = append(batch, stream)
			if len(batch) > batchSize {
				if err := process(ctx, batch); err != nil {
					return Error.Wrap(err)
				}
				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			return Error.Wrap(process(ctx, batch))
		}
		return nil
	}))
}

// processObjectStreamBatches scans and processes object streams in batches of batchSize based on the query.
func (s *SpannerAdapter) processObjectStreamBatches(ctx context.Context, asOfSystemInterval time.Duration, batchSize int, stmt spanner.Statement, process func(context.Context, []ObjectStream) error) (err error) {
	txn, err := s.client.BatchReadOnlyTransaction(ctx, spanner.StrongRead())
	if err != nil {
		return Error.Wrap(err)
	}
	defer txn.Close()

	partitions, err := txn.PartitionQueryWithOptions(ctx, stmt, spanner.PartitionOptions{
		PartitionBytes: 0,
		MaxPartitions:  0,
	}, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	batch := make([]ObjectStream, 0, batchSize)
	for _, partition := range partitions {
		iter := txn.Execute(ctx, partition)
		err := iter.Do(func(r *spanner.Row) error {
			var stream ObjectStream
			if err := r.Columns(&stream.ProjectID, &stream.BucketName, &stream.ObjectKey, &stream.Version, &stream.StreamID); err != nil {
				return Error.Wrap(err)
			}

			batch = append(batch, stream)
			if len(batch) == batchSize {
				if err := process(ctx, batch); err != nil {
					return Error.Wrap(err)
				}
				batch = batch[:0]
			}

			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}
	}

	if len(batch) > 0 {
		return Error.Wrap(process(ctx, batch))
	}

	return nil
}
