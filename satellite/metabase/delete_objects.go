// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/pgxutil"
	"storj.io/private/tagsql"
)

const (
	deleteBatchsizeLimit = intLimitRange(1000)
)

// DeleteExpiredObjects contains all the information necessary to delete expired objects and segments.
type DeleteExpiredObjects struct {
	ExpiredBefore  time.Time
	AsOfSystemTime time.Time
	BatchSize      int
}

// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
func (db *DB) DeleteExpiredObjects(ctx context.Context, opts DeleteExpiredObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
		query := `
			SELECT
				project_id, bucket_name, object_key, version, stream_id,
				expires_at
			FROM objects
			` + db.impl.AsOfSystemTime(opts.AsOfSystemTime) + `
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND expires_at < $5
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT $6;`

		expiredObjects := make([]ObjectStream, 0, batchsize)

		err = withRows(db.db.QueryContext(ctx, query,
			startAfter.ProjectID, []byte(startAfter.BucketName), []byte(startAfter.ObjectKey), startAfter.Version,
			opts.ExpiredBefore,
			batchsize),
		)(func(rows tagsql.Rows) error {
			for rows.Next() {
				var expiresAt time.Time
				err = rows.Scan(
					&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID,
					&expiresAt)
				if err != nil {
					return Error.New("unable to delete expired objects: %w", err)
				}

				db.log.Info("Deleting expired object",
					zap.Stringer("Project", last.ProjectID),
					zap.String("Bucket", last.BucketName),
					zap.String("Object Key", string(last.ObjectKey)),
					zap.Int64("Version", int64(last.Version)),
					zap.String("StreamID", hex.EncodeToString(last.StreamID[:])),
					zap.Time("Expired At", expiresAt),
				)
				expiredObjects = append(expiredObjects, last)
			}

			return nil
		})
		if err != nil {
			return ObjectStream{}, Error.New("unable to delete expired objects: %w", err)
		}

		err = db.deleteObjectsAndSegments(ctx, expiredObjects)
		if err != nil {
			return ObjectStream{}, err
		}

		return last, nil
	})
}

// DeleteZombieObjects contains all the information necessary to delete zombie objects and segments.
type DeleteZombieObjects struct {
	DeadlineBefore   time.Time
	InactiveDeadline time.Time
	AsOfSystemTime   time.Time
	BatchSize        int
}

// DeleteZombieObjects deletes all objects that zombie deletion deadline passed.
func (db *DB) DeleteZombieObjects(ctx context.Context, opts DeleteZombieObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
		query := `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			` + db.impl.AsOfSystemTime(opts.AsOfSystemTime) + `
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND status = ` + pendingStatus + `
				AND zombie_deletion_deadline < $5
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT $6;`

		objects := make([]ObjectStream, 0, batchsize)

		err = withRows(db.db.QueryContext(ctx, query,
			startAfter.ProjectID, []byte(startAfter.BucketName), []byte(startAfter.ObjectKey), startAfter.Version,
			opts.DeadlineBefore,
			batchsize),
		)(func(rows tagsql.Rows) error {
			for rows.Next() {
				err = rows.Scan(&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID)
				if err != nil {
					return Error.New("unable to delete zombie objects: %w", err)
				}

				db.log.Debug("Deleting zombie object",
					zap.Stringer("Project", last.ProjectID),
					zap.String("Bucket", last.BucketName),
					zap.String("Object Key", string(last.ObjectKey)),
					zap.Int64("Version", int64(last.Version)),
					zap.String("StreamID", hex.EncodeToString(last.StreamID[:])),
				)
				objects = append(objects, last)
			}

			return nil
		})
		if err != nil {
			return ObjectStream{}, Error.New("unable to delete zombie objects: %w", err)
		}

		err = db.deleteInactiveObjectsAndSegments(ctx, objects, opts.InactiveDeadline)
		if err != nil {
			return ObjectStream{}, err
		}

		return last, nil
	})
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

func (db *DB) deleteObjectsAndSegments(ctx context.Context, objects []ObjectStream) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return nil
	}

	err = pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch
		for _, obj := range objects {
			obj := obj

			batch.Queue(`
				WITH deleted_objects AS (
					DELETE FROM objects
					WHERE (project_id, bucket_name, object_key, version, stream_id) = ($1::BYTEA, $2, $3, $4, $5::BYTEA)
					RETURNING stream_id
				)
				DELETE FROM segments
				WHERE segments.stream_id = $5::BYTEA
			`, obj.ProjectID, []byte(obj.BucketName), []byte(obj.ObjectKey), obj.Version, obj.StreamID)
		}

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		var objectsDeletedGuess, segmentsDeleted int64

		var errlist errs.Group
		for i := 0; i < batch.Len(); i++ {
			result, err := results.Exec()
			errlist.Add(err)

			if affectedSegmentCount := result.RowsAffected(); affectedSegmentCount > 0 {
				// Note, this slightly miscounts objects without any segments
				// there doesn't seem to be a simple work around for this.
				// Luckily, this is used only for metrics, where it's not a
				// significant problem to slightly miscount.
				objectsDeletedGuess++
				segmentsDeleted += affectedSegmentCount
			}
		}

		mon.Meter("object_delete").Mark64(objectsDeletedGuess)
		mon.Meter("segment_delete").Mark64(segmentsDeleted)

		return errlist.Err()
	})
	if err != nil {
		return Error.New("unable to delete expired objects: %w", err)
	}
	return nil
}

func (db *DB) deleteInactiveObjectsAndSegments(ctx context.Context, objects []ObjectStream, inactiveDeadline time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(objects) == 0 {
		return nil
	}

	err = pgxutil.Conn(ctx, db.db, func(conn *pgx.Conn) error {
		var batch pgx.Batch
		for _, obj := range objects {
			batch.Queue(`
				WITH deleted_objects AS (
					DELETE FROM objects
					WHERE
						(project_id, bucket_name, object_key, version) = ($1::BYTEA, $2::BYTEA, $3::BYTEA, $4) AND
						stream_id = $5::BYTEA AND (
							-- TODO figure out something more optimal
							NOT EXISTS (SELECT stream_id FROM segments WHERE stream_id = $5::BYTEA)
							OR
							-- check that all segments where created before inactive time
							NOT EXISTS (SELECT stream_id FROM segments WHERE stream_id = $5::BYTEA AND created_at > $6)
						)
						RETURNING version -- return anything
				)
				DELETE FROM segments
				WHERE
					segments.stream_id = $5::BYTEA AND
					NOT EXISTS (SELECT stream_id FROM segments WHERE stream_id = $5::BYTEA AND created_at > $6)
			`, obj.ProjectID, []byte(obj.BucketName), []byte(obj.ObjectKey), obj.Version, obj.StreamID, inactiveDeadline)
		}

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		var segmentsDeleted int64
		var errlist errs.Group
		for i := 0; i < batch.Len(); i++ {
			result, err := results.Exec()
			errlist.Add(err)

			if err == nil {
				segmentsDeleted += result.RowsAffected()
			}
		}

		// TODO calculate deleted objects
		mon.Meter("zombie_segment_delete").Mark64(segmentsDeleted)
		mon.Meter("segment_delete").Mark64(segmentsDeleted)

		return errlist.Err()
	})
	if err != nil {
		return Error.New("unable to delete zombie objects: %w", err)
	}

	return nil
}
