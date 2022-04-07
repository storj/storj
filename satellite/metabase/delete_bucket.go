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

	// DeletePieces is called for every batch of objects.
	// Slice `segments` will be reused between calls.
	DeletePieces func(ctx context.Context, segments []DeletedSegmentInfo) error
}

var deleteObjectsCockroachSubSQL = `
DELETE FROM objects
WHERE project_id = $1 AND bucket_name = $2
LIMIT $3
RETURNING objects.stream_id
`

// postgres does not support LIMIT in DELETE.
var deleteObjectsPostgresSubSQL = `
DELETE FROM objects
WHERE (objects.project_id, objects.bucket_name) IN (
	SELECT project_id, bucket_name FROM objects
	WHERE project_id = $1 AND bucket_name = $2
	LIMIT $3
)
RETURNING objects.stream_id`

// TODO: remove comments with regex.
// TODO: align/merge with metabase/delete.go.
var deleteBucketObjectsWithCopyFeatureSQL = `
WITH deleted_objects AS (
	%s
),
deleted_segments AS (
	DELETE FROM segments
	WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING
		segments.stream_id,
		segments.position,
		segments.inline_data,
		segments.plain_size,
		segments.encrypted_size,
		segments.repaired_at,
		segments.root_piece_id,
		segments.remote_alias_pieces
),
deleted_copies AS (
	DELETE FROM segment_copies
	WHERE segment_copies.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING segment_copies.stream_id
),
-- lowest stream_id becomes new ancestor
promoted_ancestors AS (
	SELECT
		min(segment_copies.stream_id::text)::bytea AS new_ancestor_stream_id,
		segment_copies.ancestor_stream_id AS deleted_stream_id
	FROM segment_copies
	-- select children about to lose their ancestor
	WHERE segment_copies.ancestor_stream_id IN (
		SELECT stream_id
		FROM deleted_objects
		ORDER BY stream_id
	)
	-- don't select children which will be removed themselves
	AND segment_copies.stream_id NOT IN (
		SELECT stream_id
		FROM deleted_objects
	)
	-- select only one child to promote per ancestor
	GROUP BY segment_copies.ancestor_stream_id
)
SELECT
	deleted_objects.stream_id,
	deleted_segments.position,
	deleted_segments.root_piece_id,
	-- piece to remove from storagenodes or link to new ancestor
	deleted_segments.remote_alias_pieces,
	-- if set, caller needs to promote this stream_id to new ancestor or else object contents will be lost
	promoted_ancestors.new_ancestor_stream_id
FROM deleted_objects
LEFT JOIN deleted_segments
	ON deleted_objects.stream_id = deleted_segments.stream_id
LEFT JOIN promoted_ancestors
	ON deleted_objects.stream_id = promoted_ancestors.deleted_stream_id 
ORDER BY stream_id
`

var deleteBucketObjectsWithCopyFeaturePostgresSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectsPostgresSubSQL,
)
var deleteBucketObjectsWithCopyFeatureCockroachSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectsCockroachSubSQL,
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
	query, err := getDeleteBucketObjectsSQLWithCopyFeature(db.impl)
	if err != nil {
		return deletedObjectCount, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return deletedObjectCount, err
		}

		objects := []deletedObjectInfo{}
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

		deletedObjectCount += int64(len(objects))

		if err != nil || len(objects) == 0 {
			return deletedObjectCount, err
		}

		if opts.DeletePieces == nil {
			// no callback, should only be in test path
			continue
		}

		for _, object := range objects {
			if object.PromotedAncestor != nil {
				// don't remove pieces, they are now linked to the new ancestor
				continue
			}
			for _, segment := range object.Segments {
				// Is there an advantage to batching this?
				err := opts.DeletePieces(ctx, []DeletedSegmentInfo{
					{
						RootPieceID: segment.RootPieceID,
						Pieces:      segment.Pieces,
					},
				})
				if err != nil {
					return deletedObjectCount, err
				}
			}
		}
	}
}

func (db *DB) scanBucketObjectsDeletionServerSideCopy(ctx context.Context, location BucketLocation, rows tagsql.Rows) (result []deletedObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	defer func() { err = errs.Combine(err, rows.Close()) }()

	result = make([]deletedObjectInfo, 0, 10)
	var rootPieceID *storj.PieceID
	var object deletedObjectInfo
	var segment deletedRemoteSegmentInfo
	var aliasPieces AliasPieces
	var position *SegmentPosition

	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName

		err = rows.Scan(
			&object.StreamID,
			&position,
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
			segment.Position = *position
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

		if opts.DeletePieces != nil {
			err = opts.DeletePieces(ctx, deletedSegments)
			if err != nil {
				return deletedObjectCount, Error.Wrap(err)
			}
		}
	}
}
