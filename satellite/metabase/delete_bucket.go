// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

const (
	deleteBatchSizeLimit = intLimitRange(50)
)

// DeleteBucketObjects contains arguments for deleting a whole bucket.
type DeleteBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int
}

var deleteObjectsCockroachSubSQL = `
DELETE FROM objects
WHERE project_id = $1 AND bucket_name = $2
LIMIT $3
`

// postgres does not support LIMIT in DELETE.
var deleteObjectsPostgresSubSQL = `
DELETE FROM objects
WHERE (objects.project_id, objects.bucket_name) IN (
	SELECT project_id, bucket_name FROM objects
	WHERE project_id = $1 AND bucket_name = $2
	LIMIT $3
)`

var deleteBucketObjectsWithCopyFeaturePostgresSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectsPostgresSubSQL,
	"", "",
)
var deleteBucketObjectsWithCopyFeatureCockroachSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectsCockroachSubSQL,
	"", "",
)

func getDeleteBucketObjectsSQLWithCopyFeature(impl dbutil.Implementation) (string, error) {
	switch impl {
	case dbutil.Cockroach:
		return deleteBucketObjectsWithCopyFeatureCockroachSQL, nil
	case dbutil.Postgres:
		return deleteBucketObjectsWithCopyFeaturePostgresSQL, nil
	default:
		return "", Error.New("unhandled database: %v", impl)
	}
}

// DeleteBucketObjects deletes all objects in the specified bucket.
// Deletion performs in batches, so in case of error while processing,
// this method will return the number of objects deleted to the moment
// when an error occurs.
func (db *DB) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Bucket.Verify(); err != nil {
		return 0, err
	}

	deleteBatchSizeLimit.Ensure(&opts.BatchSize)

	if db.config.ServerSideCopy {
		return db.deleteBucketObjectsWithCopyFeatureEnabled(ctx, opts)
	}

	return db.deleteBucketObjectsWithCopyFeatureDisabled(ctx, opts)
}

func (db *DB) deleteBucketObjectsWithCopyFeatureEnabled(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		if err := ctx.Err(); err != nil {
			return deletedObjectCount, err
		}

		deletedBatchCount, err := db.deleteBucketObjectBatchWithCopyFeatureEnabled(ctx, opts)
		deletedObjectCount += deletedBatchCount

		if err != nil || deletedBatchCount == 0 {
			return deletedObjectCount, err
		}
	}
}

// deleteBucketObjectBatchWithCopyFeatureEnabled deletes a single batch from metabase.
// This function has been factored out for metric purposes.
func (db *DB) deleteBucketObjectBatchWithCopyFeatureEnabled(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query, err := getDeleteBucketObjectsSQLWithCopyFeature(db.impl)
	if err != nil {
		return 0, err
	}

	var objects []deletedObjectInfo
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		err = withRows(
			tx.QueryContext(ctx, query, opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize),
		)(func(rows tagsql.Rows) error {
			objects, err = db.scanBucketObjectsDeletionServerSideCopy(ctx, opts.Bucket, rows)
			return err
		})
		if err != nil {
			return err
		}

		return db.promoteNewAncestors(ctx, tx, objects)
	})
	if err != nil {
		return 0, err
	}

	return int64(len(objects)), err
}

func (db *DB) scanBucketObjectsDeletionServerSideCopy(ctx context.Context, location BucketLocation, rows tagsql.Rows) (result []deletedObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	defer func() { err = errs.Combine(err, rows.Close()) }()

	result = make([]deletedObjectInfo, 0, 10)
	var rootPieceID *storj.PieceID
	var object deletedObjectInfo
	var segment deletedRemoteSegmentInfo
	var aliasPieces AliasPieces
	var segmentPosition *SegmentPosition

	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName

		err = rows.Scan(
			&object.StreamID,
			&segmentPosition,
			&rootPieceID,
			&aliasPieces,
			&object.PromotedAncestor,
		)
		if err != nil {
			return nil, Error.New("unable to delete bucket objects: %w", err)
		}

		if len(result) == 0 || result[len(result)-1].StreamID != object.StreamID {
			result = append(result, object)
		}
		if rootPieceID != nil {
			segment.Position = *segmentPosition
			segment.RootPieceID = *rootPieceID
			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			if len(segment.Pieces) > 0 {
				result[len(result)-1].Segments = append(result[len(result)-1].Segments, segment)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("unable to delete object: %w", err)
	}
	return result, nil
}

func (db *DB) deleteBucketObjectsWithCopyFeatureDisabled(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string

	switch db.impl {
	case dbutil.Cockroach:
		query = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE project_id = $1 AND bucket_name = $2 LIMIT $3
			RETURNING objects.stream_id
		)
		DELETE FROM segments
		WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
		RETURNING segments.stream_id, segments.root_piece_id, segments.remote_alias_pieces
	`
	case dbutil.Postgres:
		query = `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE stream_id IN (
				SELECT stream_id FROM objects
				WHERE project_id = $1 AND bucket_name = $2
				LIMIT $3
			)
			RETURNING objects.stream_id
		)
		DELETE FROM segments
		WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
		RETURNING segments.stream_id, segments.root_piece_id, segments.remote_alias_pieces
	`
	default:
		return 0, Error.New("unhandled database: %v", db.impl)
	}

	// TODO: fix the count for objects without segments
	deletedSegments := make([]DeletedSegmentInfo, 0, 100)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		deletedSegments = deletedSegments[:0]
		deletedObjects := 0
		err = withRows(db.db.QueryContext(ctx, query,
			opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), opts.BatchSize))(func(rows tagsql.Rows) error {
			ids := map[uuid.UUID]struct{}{} // TODO: avoid map here
			for rows.Next() {
				var streamID uuid.UUID
				var segment DeletedSegmentInfo
				var aliasPieces AliasPieces
				err := rows.Scan(&streamID, &segment.RootPieceID, &aliasPieces)
				if err != nil {
					return Error.Wrap(err)
				}
				segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
				if err != nil {
					return Error.Wrap(err)
				}

				ids[streamID] = struct{}{}
				deletedSegments = append(deletedSegments, segment)
			}
			deletedObjects = len(ids)
			deletedObjectCount += int64(deletedObjects)
			return nil
		})

		mon.Meter("object_delete").Mark(deletedObjects)
		mon.Meter("segment_delete").Mark(len(deletedSegments))

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return deletedObjectCount, nil
			}
			return deletedObjectCount, Error.Wrap(err)
		}

		if len(deletedSegments) == 0 {
			return deletedObjectCount, nil
		}
	}
}
