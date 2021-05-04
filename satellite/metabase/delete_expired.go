// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil"
	"storj.io/private/tagsql"
)

const (
	deleteBatchsizeLimit = 1000
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

	var asOfSystemTimeString string
	if !opts.AsOfSystemTime.IsZero() && db.implementation == dbutil.Cockroach {
		asOfSystemTimeString = fmt.Sprintf(` AS OF SYSTEM TIME '%d' `, opts.AsOfSystemTime.UnixNano())
	}

	return db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
		query := `
			SELECT
				project_id, bucket_name, object_key, version, stream_id,
				expires_at
			FROM objects
			` + asOfSystemTimeString + `
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
	DeadlineBefore time.Time
	AsOfSystemTime time.Time
	BatchSize      int
}

// DeleteZombieObjects deletes all objects that zombie deletion deadline passed.
func (db *DB) DeleteZombieObjects(ctx context.Context, opts DeleteZombieObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	var asOfSystemTimeString string
	if !opts.AsOfSystemTime.IsZero() && db.implementation == dbutil.Cockroach {
		asOfSystemTimeString = fmt.Sprintf(` AS OF SYSTEM TIME '%d' `, opts.AsOfSystemTime.UnixNano())
	}

	return db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
		query := `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			` + asOfSystemTimeString + `
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

				db.log.Info("Deleting zombie object",
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

		err = db.deleteObjectsAndSegments(ctx, objects)
		if err != nil {
			return ObjectStream{}, err
		}

		return last, nil
	})
}

func (db *DB) deleteObjectsAndSegmentsBatch(ctx context.Context, batchsize int, deleteBatch func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error)) (err error) {
	defer mon.Task()(&ctx)(&err)

	bs := batchsize
	if batchsize == 0 || batchsize > deleteBatchsizeLimit {
		bs = deleteBatchsizeLimit
	}
	var startAfter ObjectStream
	for {
		lastDeleted, err := deleteBatch(startAfter, bs)
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

	conn, err := db.db.Conn(ctx)
	if err != nil {
		return Error.New("unable to get the raw conn: %w", err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	err = conn.Raw(ctx, func(driverConn interface{}) (err error) {
		defer func() {
			if closable, ok := driverConn.(interface{ Close() error }); ok {
				err = errs.Combine(err, closable.Close())
			}
		}()

		var pgxconn *pgx.Conn
		switch conn := driverConn.(type) {
		case interface{ StdlibConn() *stdlib.Conn }:
			pgxconn = conn.StdlibConn().Conn()
		case *stdlib.Conn:
			pgxconn = conn.Conn()
		default:
			return Error.New("invalid raw conn driver %T", driverConn)
		}

		var batch pgx.Batch
		for _, obj := range objects {
			obj := obj

			batch.Queue(`START TRANSACTION`)
			batch.Queue(`
				DELETE FROM objects
				WHERE (project_id, bucket_name, object_key, version) = ($1::BYTEA, $2::BYTEA, $3::BYTEA, $4)
					AND stream_id = $5::BYTEA
			`, obj.ProjectID, []byte(obj.BucketName), []byte(obj.ObjectKey), obj.Version, obj.StreamID)
			batch.Queue(`
				DELETE FROM segments
				WHERE segments.stream_id = $1::BYTEA
			`, obj.StreamID)
			batch.Queue(`COMMIT TRANSACTION`)
		}

		results := pgxconn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, results.Close()) }()

		var errlist errs.Group
		for i := 0; i < batch.Len(); i++ {
			_, err := results.Exec()
			errlist.Add(err)
		}

		return errlist.Err()
	})
	if err != nil {
		return Error.New("unable to delete expired objects: %w", err)
	}

	return nil
}
