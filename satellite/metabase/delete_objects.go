// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

const (
	deleteBatchsizeLimit = intLimitRange(1000)
)

// DeleteExpiredObjects contains all the information necessary to delete expired objects and segments.
type DeleteExpiredObjects struct {
	ExpiredBefore      time.Time
	AsOfSystemInterval time.Duration
	BatchSize          int
	DeleteConcurrency  int
}

// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
func (db *DB) DeleteExpiredObjects(ctx context.Context, opts DeleteExpiredObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.DeleteConcurrency == 0 {
		opts.DeleteConcurrency = 1
	}

	limiter := sync2.NewLimiter(opts.DeleteConcurrency)

	for _, a := range db.adapters {
		err = db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
			expiredObjects, err := a.FindExpiredObjects(ctx, opts, startAfter, batchsize)
			if err != nil {
				return ObjectStream{}, Error.New("unable to select expired objects for deletion: %w", err)
			}

			if len(expiredObjects) == 0 {
				return ObjectStream{}, nil
			}

			ok := limiter.Go(ctx, func() {
				objectsDeleted, segmentsDeleted, err := a.DeleteObjectsAndSegments(ctx, expiredObjects)
				if err != nil {
					db.log.Error("failed to delete expired objects from DB", zap.Error(err), zap.String("adapter", fmt.Sprintf("%T", a)))
				}

				mon.Meter("expired_object_delete").Mark64(objectsDeleted)
				mon.Meter("expired_segment_delete").Mark64(segmentsDeleted)
			})
			if !ok {
				return ObjectStream{}, Error.New("unable to start delete operation")
			}

			return expiredObjects[len(expiredObjects)-1], err
		})
		if err != nil {
			db.log.Error("failed to find expired objects in DB", zap.Error(err), zap.String("adapter", fmt.Sprintf("%T", a)))
		}
	}
	limiter.Wait()
	return nil
}

// FindExpiredObjects finds up to batchSize objects that expired before opts.ExpiredBefore.
func (p *PostgresAdapter) FindExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, startAfter ObjectStream, batchSize int) (expiredObjects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			expires_at
		FROM objects
		` + p.impl.AsOfSystemInterval(opts.AsOfSystemInterval) + `
		WHERE
			(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
			AND expires_at < $5
			ORDER BY project_id, bucket_name, object_key, version
		LIMIT $6;
	`

	expiredObjects = make([]ObjectStream, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query,
		startAfter.ProjectID, startAfter.BucketName, []byte(startAfter.ObjectKey), startAfter.Version,
		opts.ExpiredBefore,
		batchSize),
	)(func(rows tagsql.Rows) error {
		var last ObjectStream
		for rows.Next() {
			var expiresAt time.Time
			err = rows.Scan(
				&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID,
				&expiresAt)
			if err != nil {
				return Error.Wrap(err)
			}

			p.log.Debug("Deleting expired object",
				zap.Stringer("Project", last.ProjectID),
				zap.Stringer("Bucket", last.BucketName),
				zap.String("Object Key", string(last.ObjectKey)),
				zap.Int64("Version", int64(last.Version)),
				zap.Stringer("StreamID", last.StreamID),
				zap.Time("Expired At", expiresAt),
			)
			expiredObjects = append(expiredObjects, last)
		}

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return expiredObjects, nil
}

// FindExpiredObjects finds up to batchSize objects that expired before opts.ExpiredBefore.
func (s *SpannerAdapter) FindExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, startAfter ObjectStream, batchSize int) (expiredObjects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(spanner): check whether this query is executed efficiently
	expiredObjects, err = spannerutil.CollectRows(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				project_id, bucket_name, object_key, version, stream_id,
				expires_at
			FROM objects
			WHERE
				expires_at < @expires_at
				AND (
					project_id > @project_id
					OR (project_id = @project_id AND bucket_name > @bucket_name)
					OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key > @object_key)
					OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key = @object_key AND version > @version)
				)
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT @batch_size;
		`, Params: map[string]interface{}{
			"project_id":  startAfter.ProjectID,
			"bucket_name": startAfter.BucketName,
			"object_key":  startAfter.ObjectKey,
			"version":     startAfter.Version,
			"expires_at":  opts.ExpiredBefore,
			"batch_size":  batchSize,
		},
	}), func(row *spanner.Row, object *ObjectStream) error {
		var expiresAt time.Time
		err := row.Columns(
			&object.ProjectID, &object.BucketName, &object.ObjectKey, &object.Version, &object.StreamID,
			&expiresAt)
		if err != nil {
			return Error.Wrap(err)
		}

		s.log.Debug("Deleting expired object",
			zap.Stringer("Project", object.ProjectID),
			zap.Stringer("Bucket", object.BucketName),
			zap.String("Object Key", string(object.ObjectKey)),
			zap.Int64("Version", int64(object.Version)),
			zap.Stringer("StreamID", object.StreamID),
			zap.Time("Expired At", expiresAt),
		)

		return nil
	})

	return expiredObjects, Error.Wrap(err)
}

// DeleteZombieObjects contains all the information necessary to delete zombie objects and segments.
type DeleteZombieObjects struct {
	DeadlineBefore     time.Time
	InactiveDeadline   time.Time
	AsOfSystemInterval time.Duration
	BatchSize          int
}

// DeleteZombieObjects deletes all objects that zombie deletion deadline passed.
// TODO will be removed when objects table will be free from pending objects.
func (db *DB) DeleteZombieObjects(ctx context.Context, opts DeleteZombieObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, a := range db.adapters {
		err = db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
			objects, err := a.FindZombieObjects(ctx, opts, startAfter, batchsize)
			if err != nil {
				return ObjectStream{}, Error.Wrap(err)
			}

			if len(objects) == 0 {
				return ObjectStream{}, nil
			}
			objectsDeleted, segmentsDeleted, err := a.DeleteInactiveObjectsAndSegments(ctx, objects, opts)
			if err != nil {
				return ObjectStream{}, Error.Wrap(err)
			}

			mon.Meter("zombie_object_delete").Mark64(objectsDeleted)
			mon.Meter("object_delete").Mark64(objectsDeleted)
			mon.Meter("zombie_segment_delete").Mark64(segmentsDeleted)
			mon.Meter("segment_delete").Mark64(segmentsDeleted)

			return objects[len(objects)-1], nil
		})
		if err != nil {
			db.log.Warn("delete from DB zombie objects", zap.Error(err))
		}
	}
	return nil
}

// FindZombieObjects locates up to batchSize zombie objects that need deletion.
func (p *PostgresAdapter) FindZombieObjects(ctx context.Context, opts DeleteZombieObjects, startAfter ObjectStream, batchSize int) (objects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	// pending objects migrated to metabase didn't have zombie_deletion_deadline column set, because
	// of that we need to get into account also object with zombie_deletion_deadline set to NULL
	query := `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			` + p.impl.AsOfSystemInterval(opts.AsOfSystemInterval) + `
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND status = ` + statusPending + `
				AND (zombie_deletion_deadline IS NULL OR zombie_deletion_deadline < $5)
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT $6;`

	objects = make([]ObjectStream, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query,
		startAfter.ProjectID, startAfter.BucketName, []byte(startAfter.ObjectKey), startAfter.Version,
		opts.DeadlineBefore,
		batchSize),
	)(func(rows tagsql.Rows) error {
		var last ObjectStream
		for rows.Next() {
			err = rows.Scan(&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID)
			if err != nil {
				return Error.Wrap(err)
			}

			p.log.Debug("selected zombie object for deleting it",
				zap.Stringer("Project", last.ProjectID),
				zap.Stringer("Bucket", last.BucketName),
				zap.String("Object Key", string(last.ObjectKey)),
				zap.Int64("Version", int64(last.Version)),
				zap.String("StreamID", hex.EncodeToString(last.StreamID[:])),
			)
			objects = append(objects, last)
		}

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return objects, nil
}

// FindZombieObjects locates up to batchSize zombie objects that need deletion.
func (s *SpannerAdapter) FindZombieObjects(ctx context.Context, opts DeleteZombieObjects, startAfter ObjectStream, batchSize int) (objects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	// pending objects migrated to metabase didn't have zombie_deletion_deadline column set, because
	// of that we need to get into account also object with zombie_deletion_deadline set to NULL
	tuple, err := spannerutil.TupleGreaterThanSQL(
		[]string{"project_id", "bucket_name", "object_key", "version"},
		[]string{"@project_id", "@bucket_name", "@object_key", "@version"},
		false,
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	objects, err = spannerutil.CollectRows(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			WHERE
				status = ` + statusPending + `
				AND (zombie_deletion_deadline IS NULL OR zombie_deletion_deadline < @deadline)
				AND ` + tuple + `
			ORDER BY project_id, bucket_name, object_key, version
			LIMIT @batch_size
		`,
		Params: map[string]interface{}{
			"project_id":  startAfter.ProjectID,
			"bucket_name": startAfter.BucketName,
			"object_key":  startAfter.ObjectKey,
			"version":     startAfter.Version,
			"deadline":    opts.DeadlineBefore,
			"batch_size":  batchSize,
		},
	}), func(row *spanner.Row, object *ObjectStream) error {
		err := row.Columns(&object.ProjectID, &object.BucketName, &object.ObjectKey, &object.Version, &object.StreamID)
		if err != nil {
			return Error.Wrap(err)
		}

		s.log.Debug("selected zombie object for deleting it",
			zap.Stringer("Project", object.ProjectID),
			zap.Stringer("Bucket", object.BucketName),
			zap.String("Object Key", string(object.ObjectKey)),
			zap.Int64("Version", int64(object.Version)),
			zap.String("StreamID", hex.EncodeToString(object.StreamID[:])),
		)

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return objects, nil
}

func (db *DB) deleteObjectsAndSegmentsBatch(ctx context.Context, batchsize int, deleteBatch func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error)) (err error) {
	defer mon.Task()(&ctx)(&err)

	deleteBatchsizeLimit.Ensure(&batchsize)

	var startAfter ObjectStream
	for {
		lastDeleted, err := deleteBatch(startAfter, batchsize)
		if err != nil {
			return err
		}
		if lastDeleted.StreamID.IsZero() {
			return nil
		}
		startAfter = lastDeleted
	}
}

// DeleteObjectsAndSegments deletes expired objects and associated segments.
func (p *PostgresAdapter) DeleteObjectsAndSegments(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error) {
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

// DeleteObjectsAndSegments deletes expired objects and associated segments.
func (s *SpannerAdapter) DeleteObjectsAndSegments(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return 0, 0, nil
	}

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// can't use Mutations here, since we only want to delete objects by the specified keys
		// if and only if the stream_id matches.
		var statements []spanner.Statement
		for _, obj := range objects {
			obj := obj
			statements = append(statements, spanner.Statement{
				SQL: `
					DELETE FROM objects
					WHERE (project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id)
				`,
				Params: map[string]interface{}{
					"project_id":  obj.ProjectID,
					"bucket_name": obj.BucketName,
					"object_key":  obj.ObjectKey,
					"version":     obj.Version,
					"stream_id":   obj.StreamID,
				},
			})
		}
		numDeleteds, err := tx.BatchUpdate(ctx, statements)
		if err != nil {
			return Error.Wrap(err)
		}
		for _, numDeleted := range numDeleteds {
			objectsDeleted += numDeleted
		}

		stmts := make([]spanner.Statement, len(objects))
		for ix, object := range objects {
			stmts[ix] = spanner.Statement{
				SQL: `DELETE FROM segments WHERE @stream_id = stream_id`,
				Params: map[string]interface{}{
					"stream_id": object.StreamID.Bytes(),
				},
			}
		}
		if len(stmts) > 0 {
			numSegments, err := tx.BatchUpdate(ctx, stmts)
			if err != nil {
				return Error.Wrap(err)
			}
			for _, v := range numSegments {
				segmentsDeleted += v
			}
		}
		return nil
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

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
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

		numDeleteds, err := tx.BatchUpdate(ctx, statements)
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

		numSegments, err := tx.Update(ctx, spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE ARRAY_INCLUDES(@stream_ids, stream_id)
			`,
			Params: map[string]interface{}{
				"stream_ids": streamIDs,
			},
		})
		if err != nil {
			return Error.Wrap(err)
		}
		segmentsDeleted += numSegments
		return nil
	})
	if err != nil {
		return objectsDeleted, segmentsDeleted, Error.New("unable to delete zombie objects: %w", err)
	}
	return objectsDeleted, segmentsDeleted, nil
}
