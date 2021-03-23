// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
)

const (
	expiredBatchsizeLimit = 1000
)

// DeleteExpiredObjects contains all the information necessary to delete expired objects and segments.
type DeleteExpiredObjects struct {
	ExpiredBefore  time.Time
	AsOfSystemTime time.Time
	BatchSize      int
}

type expiredObject struct {
	projectID  uuid.UUID
	bucketName string
	objectKey  ObjectKey
	version    Version
	streamID   uuid.UUID
}

// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
func (db *DB) DeleteExpiredObjects(ctx context.Context, opts DeleteExpiredObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	batchsize := opts.BatchSize
	if opts.BatchSize == 0 || opts.BatchSize > expiredBatchsizeLimit {
		batchsize = expiredBatchsizeLimit
	}
	var startAfter Object
	for {
		lastDeleted, err := db.deleteExpiredObjectsBatch(ctx, startAfter, opts.ExpiredBefore, opts.AsOfSystemTime, batchsize)
		if err != nil {
			return err
		}
		if lastDeleted.StreamID.IsZero() {
			return nil
		}
		startAfter = lastDeleted
	}
}

func (db *DB) deleteExpiredObjectsBatch(ctx context.Context, startAfter Object, expiredBefore time.Time, asOfSystemTime time.Time, batchsize int) (lastDeleted Object, err error) {
	defer mon.Task()(&ctx)(&err)

	var asOfSystemTimeString string
	if !asOfSystemTime.IsZero() && db.implementation == dbutil.Cockroach {

		asOfSystemTimeString = fmt.Sprintf(` AS OF SYSTEM TIME '%d' `, asOfSystemTime.Add(1*time.Second).UTC().UnixNano())
	}
	query := `
			SELECT project_id, bucket_name,
			object_key, version, stream_id,
			expires_at
			FROM objects
			` + asOfSystemTimeString + `
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND expires_at < $5
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT $6;`

	expiredObjects := make([]expiredObject, 0, batchsize)

	err = withRows(db.db.QueryContext(ctx, query, lastDeleted.ProjectID, []byte(lastDeleted.BucketName), []byte(lastDeleted.ObjectKey), lastDeleted.Version,
		expiredBefore,
		batchsize),
	)(func(rows tagsql.Rows) error {
		for rows.Next() {
			err = rows.Scan(&lastDeleted.ProjectID, &lastDeleted.BucketName,
				&lastDeleted.ObjectKey, &lastDeleted.Version, &lastDeleted.StreamID,
				&lastDeleted.ExpiresAt)
			if err != nil {
				return Error.New("unable to delete expired objects: %w", err)
			}

			db.log.Info("Deleting expired object",
				zap.Stringer("Project", lastDeleted.ProjectID),
				zap.String("Bucket", lastDeleted.BucketName),
				zap.String("Object Key", string(lastDeleted.ObjectKey)),
				zap.Int64("Version", int64(lastDeleted.Version)),
				zap.Time("Expired At", *lastDeleted.ExpiresAt),
			)
			expiredObjects = append(expiredObjects, expiredObject{
				lastDeleted.ProjectID, lastDeleted.BucketName, lastDeleted.ObjectKey, lastDeleted.Version,
				lastDeleted.StreamID,
			})
		}

		return nil
	})
	if err != nil {
		return Object{}, Error.New("unable to delete expired objects: %w", err)
	}

	err = db.deleteExpiredObjects(ctx, expiredObjects)
	if err != nil {
		return Object{}, err
	}

	if err != nil {
		return Object{}, err
	}

	return lastDeleted, nil
}

func (db *DB) deleteExpiredObjects(ctx context.Context, expiredObjects []expiredObject) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(expiredObjects) == 0 {
		return nil
	}

	projectIds := make([]uuid.UUID, len(expiredObjects))
	buckets := make([][]byte, len(expiredObjects))
	objectKeys := make([][]byte, len(expiredObjects))
	versions := make([]int32, len(expiredObjects))
	streamIds := make([][]byte, len(expiredObjects))

	for i, object := range expiredObjects {
		projectIds[i] = object.projectID
		buckets[i] = []byte(object.bucketName)
		objectKeys[i] = []byte(object.objectKey)
		versions[i] = int32(object.version)
		streamIds[i] = object.streamID[:]
	}
	query := `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key, version, stream_id) IN (
					SELECT
						unnest($1::BYTEA[]),
						unnest($2::BYTEA[]),
						unnest($3::BYTEA[]),
						unnest($4::INT4[]),
						unnest($5::BYTEA[])
				)
			RETURNING stream_id
		)
		DELETE FROM segments
		WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects ORDER BY deleted_objects.stream_id)
	`
	_, err = db.db.ExecContext(ctx,
		query,
		pgutil.UUIDArray(projectIds),
		pgutil.ByteaArray(buckets),
		pgutil.ByteaArray(objectKeys),
		pgutil.Int4Array(versions),
		pgutil.ByteaArray(streamIds),
	)

	if err != nil {
		return Error.New("unable to delete expired objects: %w", err)
	}
	return nil
}
