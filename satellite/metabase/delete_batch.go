// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/tagsql"
)

// deleteObjectsBatchLimit bounds how many objects a single batch-delete call
// will accept.
const deleteObjectsBatchLimit = 1000

// DeleteObjectsAndSegmentsNoVerify contains arguments for deleting a batch of
// objects and their segments without verifying that the supplied stream ids
// belong to the objects.
type DeleteObjectsAndSegmentsNoVerify struct {
	Objects []ObjectStream
}

// Verify verifies the request fields.
func (opts DeleteObjectsAndSegmentsNoVerify) Verify() error {
	if len(opts.Objects) > deleteObjectsBatchLimit {
		return ErrInvalidRequest.New("too many objects to delete; expected <= %d, but got %d", deleteObjectsBatchLimit, len(opts.Objects))
	}
	return nil
}

// DeleteInactiveObjectsAndSegments contains arguments for deleting a batch of
// inactive objects and their segments.
type DeleteInactiveObjectsAndSegments struct {
	Objects          []ObjectStream
	InactiveDeadline time.Time
}

// Verify verifies the request fields.
func (opts DeleteInactiveObjectsAndSegments) Verify() error {
	if len(opts.Objects) > deleteObjectsBatchLimit {
		return ErrInvalidRequest.New("too many objects to delete; expected <= %d, but got %d", deleteObjectsBatchLimit, len(opts.Objects))
	}
	return nil
}

// DeleteObjectsAndSegmentsNoVerify deletes expired objects and associated segments.
func (p *PostgresAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, opts DeleteObjectsAndSegmentsNoVerify) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
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
func (t *TiDBAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, opts DeleteObjectsAndSegmentsNoVerify) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
	if len(objects) == 0 {
		return 0, 0, nil
	}

	err = tidbutil.WithRawTx(ctx, t.db, func(ctx context.Context, tx tidbutil.RawTx) error {
		objectsDeleted, segmentsDeleted = 0, 0

		// The batch is bounded by deleteObjectsBatchLimit (well under
		// tidbMaxSegmentBatch), so both DELETEs run in one Exec.
		var sb strings.Builder
		args := make([]any, 0, len(objects)*6)

		sb.WriteString(`DELETE FROM objects WHERE (project_id, bucket_name, object_key, version, stream_id) IN (` +
			strings.Repeat("(?,?,?,?,?),", len(objects)-1) + `(?,?,?,?,?));`)
		for _, obj := range objects {
			args = append(args, obj.ProjectID, []byte(obj.BucketName), []byte(obj.ObjectKey), int64(obj.Version), obj.StreamID.Bytes())
		}

		sb.WriteString(`DELETE FROM segments WHERE stream_id IN (` + tidbPlaceholders(len(objects)) + `);`)
		for _, obj := range objects {
			args = append(args, obj.StreamID.Bytes())
		}

		res, err := tx.ExecContext(ctx, sb.String(), args...)
		if err != nil {
			return Error.New("unable to delete expired objects: %w", err)
		}
		counts := res.AllRowsAffected()
		if len(counts) != 2 {
			return Error.New("driver returned %d row-affected counts, expected 2", len(counts))
		}
		segmentsDeleted = counts[1] // counts[0] is the object DELETE.
		// Approximate the object count like the Postgres adapter does.
		if segmentsDeleted > 0 {
			objectsDeleted = int64(len(objects))
		}
		return nil
	})
	return objectsDeleted, segmentsDeleted, err
}

// DeleteObjectsAndSegmentsNoVerify deletes expired objects and associated segments.
//
// The implementation does not do extra verification whether the stream id-s belong or belonged to the objects.
// So, if the callers supplies objects with incorrect StreamID-s it may end up deleting unrelated segments.
func (s *SpannerAdapter) DeleteObjectsAndSegmentsNoVerify(ctx context.Context, opts DeleteObjectsAndSegmentsNoVerify) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
	if len(objects) == 0 {
		return 0, 0, nil
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		objectsDeleted = 0
		segmentsDeleted = 0

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
func (p *PostgresAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, opts DeleteInactiveObjectsAndSegments) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
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
		p.log.Warn("unable to delete zombie objects and segments", zap.Error(err))
		return 0, 0, nil
	}
	return objectsDeleted, segmentsDeleted, nil
}

// DeleteInactiveObjectsAndSegments deletes inactive objects and associated segments.
func (t *TiDBAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, opts DeleteInactiveObjectsAndSegments) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
	if len(objects) == 0 {
		return 0, 0, nil
	}

	// Deleting happens in two phases inside a single transaction. TiDB has no
	// DELETE ... RETURNING, so we can't delete an object and its segments in
	// one statement and still learn which objects were removed:
	//
	//   1. One DELETE per object, matching the full primary key and skipping
	//      the object when one of its segments was created after the deadline.
	//      The per-statement row counts report which objects were deleted.
	//   2. Delete the segments of the objects removed in phase 1, by stream_id.
	//
	// The batch is bounded by deleteObjectsBatchLimit (well under
	// tidbMaxSegmentBatch), so each phase runs as a single Exec.
	err = tidbutil.WithRawTx(ctx, t.db, func(ctx context.Context, tx tidbutil.RawTx) error {
		objectsDeleted, segmentsDeleted = 0, 0

		const deleteObjectSQL = `DELETE FROM objects
			WHERE (project_id, bucket_name, object_key, version, stream_id) = (?,?,?,?,?)
			  AND NOT EXISTS (
				SELECT 1 FROM segments
				WHERE segments.stream_id = ? AND segments.created_at > ?
			  );`

		args := make([]any, 0, len(objects)*7)
		for _, obj := range objects {
			sid := obj.StreamID.Bytes()
			args = append(args,
				obj.ProjectID, []byte(obj.BucketName), []byte(obj.ObjectKey), int64(obj.Version), sid,
				sid, opts.InactiveDeadline,
			)
		}
		res, err := tx.ExecContext(ctx, strings.Repeat(deleteObjectSQL, len(objects)), args...)
		if err != nil {
			return Error.New("unable to delete inactive objects: %w", err)
		}
		counts := res.AllRowsAffected()
		if len(counts) != len(objects) {
			return Error.New("driver returned %d row-affected counts, expected %d", len(counts), len(objects))
		}

		segmentArgs := make([]any, 0, len(objects))
		for i, c := range counts {
			if c > 0 {
				objectsDeleted++
				segmentArgs = append(segmentArgs, objects[i].StreamID.Bytes())
			}
		}
		if len(segmentArgs) == 0 {
			return nil
		}

		res, err = tx.ExecContext(ctx,
			`DELETE FROM segments WHERE stream_id IN (`+tidbPlaceholders(len(segmentArgs))+`);`,
			segmentArgs...)
		if err != nil {
			return Error.New("unable to delete inactive segments: %w", err)
		}
		segmentsDeleted, err = res.RowsAffected()
		if err != nil {
			return Error.New("unable to delete inactive segments: %w", err)
		}
		return nil
	})
	if err != nil {
		// Mirror the Postgres adapter's behavior: log and swallow.
		t.log.Warn("unable to delete zombie objects and segments", zap.Error(err))
		return 0, 0, nil
	}
	return objectsDeleted, segmentsDeleted, nil
}

// DeleteInactiveObjectsAndSegments deletes inactive objects and associated segments.
func (s *SpannerAdapter) DeleteInactiveObjectsAndSegments(ctx context.Context, opts DeleteInactiveObjectsAndSegments) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	objects := opts.Objects
	if len(objects) == 0 {
		return 0, 0, nil
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// Reset counters in case the transaction is retried.
		objectsDeleted = 0
		segmentsDeleted = 0

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
				Params: map[string]any{
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
			Params: map[string]any{
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
		s.log.Warn("unable to delete zombie objects and segments", zap.Error(err))
		return 0, 0, nil
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
			if len(batch) >= batchSize {
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
func (t *TiDBAdapter) processObjectStreamBatches(ctx context.Context, batchSize int, stmt postgresStatement, process func(context.Context, []ObjectStream) error) (err error) {
	return Error.Wrap(withRows(
		t.db.QueryContext(ctx, stmt.SQL, stmt.Params...),
	)(func(rows tagsql.Rows) error {
		batch := make([]ObjectStream, 0, batchSize)
		for rows.Next() {
			var stream ObjectStream
			if err := rows.Scan(&stream.ProjectID, &stream.BucketName, &stream.ObjectKey, &stream.Version, &stream.StreamID); err != nil {
				return Error.Wrap(err)
			}
			batch = append(batch, stream)
			if len(batch) >= batchSize {
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
