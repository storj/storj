// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
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
	deleteBatchsizeLimit  = intLimitRange(1000)
	deleteObjectsMaxItems = 1000
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
				objectsDeleted, segmentsDeleted, err := a.DeleteObjectsAndSegmentsNoVerify(ctx, expiredObjects)
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

	// TODO: make util for using stale reads
	transaction := s.client.Single()
	if opts.AsOfSystemInterval != 0 {
		// spanner requires non-negative staleness
		staleness := opts.AsOfSystemInterval
		if staleness < 0 {
			staleness *= -1
		}

		transaction = transaction.WithTimestampBound(spanner.MaxStaleness(staleness))
	}

	// TODO(spanner): check whether this query is executed efficiently
	expiredObjects, err = spannerutil.CollectRows(transaction.QueryWithOptions(ctx, spanner.Statement{
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
	}, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
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

	objects, err = spannerutil.CollectRows(s.client.Single().QueryWithOptions(ctx, spanner.Statement{
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
	}, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
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
		CommitPriority: spannerpb.RequestOptions_PRIORITY_LOW,
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
		CommitPriority: spannerpb.RequestOptions_PRIORITY_LOW,
	})
	if err != nil {
		return objectsDeleted, segmentsDeleted, Error.New("unable to delete zombie objects: %w", err)
	}
	return objectsDeleted, segmentsDeleted, nil
}

// DeleteObjects contains options for deleting multiple committed objects from a bucket.
type DeleteObjects struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	Items      []DeleteObjectsItem

	Versioned bool
	Suspended bool

	ObjectLock ObjectLockDeleteOptions
}

// DeleteObjectsItem describes the location of an object in a bucket to be deleted.
type DeleteObjectsItem struct {
	ObjectKey       ObjectKey
	StreamVersionID StreamVersionID
}

// Verify verifies bucket object deletion request fields.
func (opts DeleteObjects) Verify() error {
	itemCount := len(opts.Items)
	switch {
	case opts.Suspended:
		return ErrInvalidRequest.New("deletion from buckets with versioning suspended is not yet supported")
	case opts.ObjectLock.Enabled:
		return ErrInvalidRequest.New("deletion from buckets with Object Lock enabled is not yet supported")
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case itemCount == 0:
		return ErrInvalidRequest.New("Items missing")
	case itemCount > deleteObjectsMaxItems:
		return ErrInvalidRequest.New("Items is too long; expected <= %d, but got %d", deleteObjectsMaxItems, itemCount)
	}
	for i, item := range opts.Items {
		if item.ObjectKey == "" {
			return ErrInvalidRequest.New("Items[%d].ObjectKey missing", i)
		}
		version := item.StreamVersionID.Version()
		if !item.StreamVersionID.IsZero() && version <= 0 {
			return ErrInvalidRequest.New("Items[%d].StreamVersionID invalid: version is %v", i, version)
		}
	}
	return nil
}

// DeleteObjectsResult contains the results of an attempt to delete specific objects from a bucket.
type DeleteObjectsResult struct {
	Items               []DeleteObjectsResultItem
	DeletedSegmentCount int64
}

// DeleteObjectsStatus represents the success or failure status of an individual DeleteObjects deletion.
type DeleteObjectsStatus int

const (
	// DeleteStatusNotFound indicates that the object could not be deleted because it didn't exist.
	DeleteStatusNotFound DeleteObjectsStatus = iota
	// DeleteStatusOK indicates that the object was successfully deleted.
	DeleteStatusOK
	// DeleteStatusInternalError indicates that an internal error occurred when attempting to delete the object.
	DeleteStatusInternalError
)

// DeleteObjectsResultItem contains the result of an attempt to delete a specific object from a bucket.
type DeleteObjectsResultItem struct {
	ObjectKey                ObjectKey
	RequestedStreamVersionID StreamVersionID

	Removed *DeleteObjectsInfo
	Marker  *DeleteObjectsInfo

	Status DeleteObjectsStatus
}

// DeleteObjectsInfo contains information about an object that was deleted or a delete marker that was inserted
// as a result of processing a DeleteObjects request item.
type DeleteObjectsInfo struct {
	StreamVersionID StreamVersionID
	Status          ObjectStatus
}

// DeleteObjects deletes specific objects from a bucket.
//
// TODO: Support Object Lock and properly handle buckets with versioning suspended.
func (db *DB) DeleteObjects(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectsResult{}, errs.Wrap(err)
	}

	if opts.Versioned {
		result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectsVersioned(ctx, opts)
	} else {
		result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectsPlain(ctx, opts)
	}
	if err != nil {
		return DeleteObjectsResult{}, errs.Wrap(err)
	}

	var deletedObjects int
	for _, item := range result.Items {
		if item.Status == DeleteStatusOK && item.Removed != nil {
			deletedObjects++
		}
	}
	if deletedObjects > 0 {
		mon.Meter("object_delete").Mark(deletedObjects)
	}
	if result.DeletedSegmentCount > 0 {
		mon.Meter("segment_delete").Mark64(result.DeletedSegmentCount)
	}

	return result, nil
}

type deleteObjectsSetupInfo struct {
	results        []DeleteObjectsResultItem
	resultsIndices map[DeleteObjectsItem]int
}

// processResults returns data that (*Adapter).DeleteObjects implementations require for executing database queries.
func (opts DeleteObjects) processResults() (info deleteObjectsSetupInfo) {
	info.resultsIndices = make(map[DeleteObjectsItem]int, len(opts.Items))
	i := 0
	for _, item := range opts.Items {
		if _, exists := info.resultsIndices[item]; !exists {
			info.resultsIndices[item] = i
			i++
		}
	}

	info.results = make([]DeleteObjectsResultItem, len(info.resultsIndices))
	for item, resultsIdx := range info.resultsIndices {
		info.results[resultsIdx] = DeleteObjectsResultItem{
			ObjectKey:                item.ObjectKey,
			RequestedStreamVersionID: item.StreamVersionID,
		}
	}

	return info
}

// DeleteObjectsPlain deletes specific objects from an unversioned bucket.
func (p *PostgresAdapter) DeleteObjectsPlain(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now().Truncate(time.Microsecond)

	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]

		if resultItem.RequestedStreamVersionID.IsZero() {
			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key) = ($1, $2, $3)
							AND status = `+statusCommittedUnversioned+`
							AND (expires_at IS NULL OR expires_at > $4)
						RETURNING version, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT version, stream_id, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					version      Version
					streamID     uuid.UUID
					segmentCount int64
				)
				if err := rows.Scan(&version, &streamID, &segmentCount); err != nil {
					return errs.Wrap(err)
				}

				result.DeletedSegmentCount += segmentCount

				sv := NewStreamVersionID(version, streamID)
				deleteInfo := &DeleteObjectsInfo{
					StreamVersionID: sv,
					Status:          CommittedUnversioned,
				}
				resultItem.Removed = deleteInfo
				resultItem.Status = DeleteStatusOK

				// Handle the case where an object was specified twice in the deletion request:
				// once with a version omitted and once with a version set. We must ensure that
				// when the object is deleted, both result items that reference it are updated.
				if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
					ObjectKey:       resultItem.ObjectKey,
					StreamVersionID: sv,
				}]; ok {
					processedOpts.results[i].Removed = deleteInfo
					processedOpts.results[i].Status = DeleteStatusOK
				}

				if rows.Next() {
					logMultipleCommittedVersionsError(p.log, ObjectLocation{
						ProjectID:  opts.ProjectID,
						BucketName: opts.BucketName,
						ObjectKey:  resultItem.ObjectKey,
					})
				}

				return nil
			}))
		} else {
			if resultItem.Status == DeleteStatusOK {
				continue
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
							AND SUBSTR(stream_id, 9) = $5
							AND (expires_at IS NULL OR expires_at > $6)
						RETURNING status, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT status, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					resultItem.RequestedStreamVersionID.Version(),
					resultItem.RequestedStreamVersionID.StreamIDSuffix(),
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					status       ObjectStatus
					segmentCount int64
				)
				if err := rows.Scan(&status, &segmentCount); err != nil {
					return errs.Wrap(err)
				}

				result.DeletedSegmentCount += segmentCount

				deleteInfo := &DeleteObjectsInfo{
					StreamVersionID: resultItem.RequestedStreamVersionID,
					Status:          status,
				}
				resultItem.Removed = deleteInfo
				resultItem.Status = DeleteStatusOK

				if status == CommittedUnversioned {
					if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
						ObjectKey: resultItem.ObjectKey,
					}]; ok {
						processedOpts.results[i].Removed = deleteInfo
						processedOpts.results[i].Status = DeleteStatusOK
					}
				}

				return nil
			}))
		}

		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, err
}

func spannerDeleteSegmentsByStreamID(ctx context.Context, tx *spanner.ReadWriteTransaction, streamIDs [][]byte) (count int64, err error) {
	if len(streamIDs) == 0 {
		return 0, nil
	}
	count, err = tx.Update(ctx, spanner.Statement{
		SQL: `
			DELETE FROM segments
			WHERE stream_id IN UNNEST(@stream_ids)
		`,
		Params: map[string]interface{}{
			"stream_ids": streamIDs,
		},
	})
	return count, errs.Wrap(err)
}

// DeleteObjectsPlain deletes the specified objects from an unversioned bucket.
func (s *SpannerAdapter) DeleteObjectsPlain(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now().Truncate(time.Microsecond)

	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]

		var (
			deletedSegmentCount int64
			linkedResultItem    *DeleteObjectsResultItem
		)

		if resultItem.RequestedStreamVersionID.IsZero() {
			var multipleCommittedVersions bool
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				deletedSegmentCount = 0
				multipleCommittedVersions = false
				linkedResultItem = nil

				var streamIDsToDelete [][]byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
							AND status = ` + statusCommittedUnversioned + `
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN version, stream_id
					`,
					Params: map[string]interface{}{
						"project_id":  opts.ProjectID,
						"bucket_name": opts.BucketName,
						"object_key":  resultItem.ObjectKey,
						"now":         now,
					},
				}).Do(func(row *spanner.Row) error {
					var (
						version       Version
						streamIDBytes []byte
					)
					if err := row.Columns(&version, &streamIDBytes); err != nil {
						return errs.Wrap(err)
					}

					streamIDsToDelete = append(streamIDsToDelete, streamIDBytes)

					if resultItem.Removed != nil {
						multipleCommittedVersions = true
						return nil
					}

					streamID, err := uuid.FromBytes(streamIDBytes)
					if err != nil {
						return errs.Wrap(err)
					}

					sv := NewStreamVersionID(version, streamID)
					deleteInfo := &DeleteObjectsInfo{
						StreamVersionID: sv,
						Status:          CommittedUnversioned,
					}
					resultItem.Removed = deleteInfo
					resultItem.Status = DeleteStatusOK

					// Handle the case where an object was specified twice in the deletion request:
					// once with a version omitted and once with a version set. We must ensure that
					// when the object is deleted, both deletion results that reference it are updated.
					if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
						ObjectKey:       resultItem.ObjectKey,
						StreamVersionID: sv,
					}]; ok {
						linkedResultItem = &processedOpts.results[i]
						linkedResultItem.Removed = deleteInfo
						linkedResultItem.Status = DeleteStatusOK
					}

					return nil
				}))
				if err != nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, streamIDsToDelete)
				return err
			})
			if err == nil && multipleCommittedVersions {
				logMultipleCommittedVersionsError(s.log, ObjectLocation{
					ProjectID:  opts.ProjectID,
					BucketName: opts.BucketName,
					ObjectKey:  resultItem.ObjectKey,
				})
			}
		} else {
			if resultItem.Status == DeleteStatusOK {
				continue
			}

			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				deletedSegmentCount = 0
				linkedResultItem = nil

				var streamID []byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
							AND SUBSTR(stream_id, 9) = @stream_id_suffix
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN status, stream_id
					`,
					Params: map[string]interface{}{
						"project_id":       opts.ProjectID,
						"bucket_name":      opts.BucketName,
						"object_key":       resultItem.ObjectKey,
						"version":          resultItem.RequestedStreamVersionID.Version(),
						"stream_id_suffix": resultItem.RequestedStreamVersionID.StreamIDSuffix(),
						"now":              now,
					},
				}).Do(func(row *spanner.Row) error {
					var status ObjectStatus
					if err := row.Columns(&status, &streamID); err != nil {
						return errs.Wrap(err)
					}

					deleteInfo := &DeleteObjectsInfo{
						StreamVersionID: resultItem.RequestedStreamVersionID,
						Status:          status,
					}
					resultItem.Removed = deleteInfo
					resultItem.Status = DeleteStatusOK

					if status == CommittedUnversioned {
						if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
							ObjectKey: resultItem.ObjectKey,
						}]; ok {
							linkedResultItem = &processedOpts.results[i]
							linkedResultItem.Removed = deleteInfo
							linkedResultItem.Status = DeleteStatusOK
						}
					}

					return nil
				}))
				if err != nil || resultItem.Removed == nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, [][]byte{streamID})
				return err
			})
		}

		if err == nil {
			result.DeletedSegmentCount += deletedSegmentCount
		} else {
			resultItem.Removed = nil
			if linkedResultItem != nil {
				linkedResultItem.Removed = nil
			}
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, Error.Wrap(err)
}

// DeleteObjectsVersioned deletes specific objects from a bucket with versioning enabled.
func (p *PostgresAdapter) DeleteObjectsVersioned(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now()
	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]
		if resultItem.RequestedStreamVersionID.IsZero() {
			var streamID uuid.UUID
			streamID, err = generateDeleteMarkerStreamID()
			if err != nil {
				break
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					INSERT INTO objects (
						project_id, bucket_name, object_key, version, stream_id,
						status,
						zombie_deletion_deadline
					)
					SELECT
						$1, $2, $3,
							coalesce((
								SELECT version + 1
								FROM objects
								WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
								ORDER BY version DESC
								LIMIT 1
							), 1),
						$4,
						`+statusDeleteMarkerVersioned+`,
						NULL
					RETURNING version`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					streamID,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return errs.New("could not insert delete marker")
				}

				var version Version
				if err := rows.Scan(&version); err != nil {
					return errs.Wrap(err)
				}

				resultItem.Status = DeleteStatusOK
				resultItem.Marker = &DeleteObjectsInfo{
					StreamVersionID: NewStreamVersionID(version, streamID),
					Status:          DeleteMarkerVersioned,
				}

				return nil
			}))
		} else {
			// Prevent the removal of a delete marker that was added in a previous iteration.
			if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
				ObjectKey: resultItem.ObjectKey,
			}]; ok {
				marker := processedOpts.results[i].Marker
				if marker != nil && marker.StreamVersionID == resultItem.RequestedStreamVersionID {
					continue
				}
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
							AND SUBSTR(stream_id, 9) = $5
							AND (expires_at IS NULL OR expires_at > $6)
						RETURNING status, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT status, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					resultItem.RequestedStreamVersionID.Version(),
					resultItem.RequestedStreamVersionID.StreamIDSuffix(),
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					status       ObjectStatus
					segmentCount int64
				)
				if err := rows.Scan(&status, &segmentCount); err != nil {
					return errs.Wrap(err)
				}
				result.DeletedSegmentCount += segmentCount

				resultItem.Status = DeleteStatusOK
				resultItem.Removed = &DeleteObjectsInfo{
					StreamVersionID: resultItem.RequestedStreamVersionID,
					Status:          status,
				}

				return nil
			}))
		}
		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, err
}

// DeleteObjectsVersioned deletes specific objects from a bucket with versioning enabled.
func (s *SpannerAdapter) DeleteObjectsVersioned(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now()
	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]
		if resultItem.RequestedStreamVersionID.IsZero() {
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				resultItem.Marker = nil
				resultItem.Status = DeleteStatusNotFound

				var streamID uuid.UUID
				streamID, err = generateDeleteMarkerStreamID()
				if err != nil {
					return err
				}

				return errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						INSERT INTO objects (
							project_id, bucket_name, object_key, version, stream_id,
							status,
							zombie_deletion_deadline
						)
						SELECT
							@project_id, @bucket_name, @object_key,
								coalesce((
									SELECT version + 1
									FROM objects
									WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
									ORDER BY version DESC
									LIMIT 1
								), 1),
							@stream_id,
							` + statusDeleteMarkerVersioned + `,
							NULL
						THEN RETURN version`,
					Params: map[string]interface{}{
						"project_id":  opts.ProjectID,
						"bucket_name": opts.BucketName,
						"object_key":  resultItem.ObjectKey,
						"stream_id":   streamID,
					},
				}).Do(func(row *spanner.Row) error {
					var version Version
					if err := row.Columns(&version); err != nil {
						return errs.Wrap(err)
					}

					resultItem.Marker = &DeleteObjectsInfo{
						StreamVersionID: NewStreamVersionID(version, streamID),
						Status:          DeleteMarkerVersioned,
					}
					resultItem.Status = DeleteStatusOK

					return nil
				}))
			})
			if err != nil {
				resultItem.Marker = nil
			}
		} else {
			// Prevent the removal of a delete marker that was added in a previous iteration.
			if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
				ObjectKey: resultItem.ObjectKey,
			}]; ok {
				marker := processedOpts.results[i].Marker
				if marker != nil && marker.StreamVersionID == resultItem.RequestedStreamVersionID {
					continue
				}
			}

			var deletedSegmentCount int64
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				resultItem.Status = DeleteStatusNotFound
				deletedSegmentCount = 0

				var streamID []byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
							AND SUBSTR(stream_id, 9) = @stream_id_suffix
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN status, stream_id`,
					Params: map[string]interface{}{
						"project_id":       opts.ProjectID,
						"bucket_name":      opts.BucketName,
						"object_key":       resultItem.ObjectKey,
						"version":          resultItem.RequestedStreamVersionID.Version(),
						"stream_id_suffix": resultItem.RequestedStreamVersionID.StreamIDSuffix(),
						"now":              now,
					},
				}).Do(func(row *spanner.Row) error {
					var status ObjectStatus
					if err := row.Columns(&status, &streamID); err != nil {
						return errs.Wrap(err)
					}

					resultItem.Status = DeleteStatusOK
					resultItem.Removed = &DeleteObjectsInfo{
						StreamVersionID: resultItem.RequestedStreamVersionID,
						Status:          status,
					}

					return nil
				}))
				if err != nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, [][]byte{streamID})
				return err
			})
			if err == nil {
				result.DeletedSegmentCount += deletedSegmentCount
			} else {
				resultItem.Removed = nil
			}
		}
		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, Error.Wrap(err)
}
