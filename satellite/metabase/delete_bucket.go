// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/tagsql"
)

const deleteBatchSizeLimit = 100

// DeleteBucketObjects contains arguments for deleting a whole bucket.
type DeleteBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int

	// DeletePieces is called for every batch of objects.
	// Slice `segments` will be reused between calls.
	DeletePieces func(ctx context.Context, segments []DeletedSegmentInfo) error
}

// DeleteBucketObjects deletes all objects in the specified bucket.
func (db *DB) DeleteBucketObjects(ctx context.Context, opts DeleteBucketObjects) (deletedObjectCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Bucket.Verify(); err != nil {
		return 0, err
	}

	batchSize := opts.BatchSize
	if batchSize <= 0 || batchSize > deleteBatchSizeLimit {
		batchSize = deleteBatchSizeLimit
	}

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
		WHERE segments.stream_id in (SELECT deleted_objects.stream_id FROM deleted_objects)
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
		WHERE segments.stream_id in (SELECT deleted_objects.stream_id FROM deleted_objects)
		RETURNING segments.stream_id, segments.root_piece_id, segments.remote_alias_pieces
	`
	default:
		return deletedObjectCount, Error.New("unhandled database: %v", db.impl)
	}

	// TODO: fix the count for objects without segments

	var deleteSegments []DeletedSegmentInfo
	for {
		deleteSegments = deleteSegments[:0]
		err = withRows(db.db.Query(ctx, query,
			opts.Bucket.ProjectID, []byte(opts.Bucket.BucketName), batchSize))(func(rows tagsql.Rows) error {
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
				deleteSegments = append(deleteSegments, segment)
			}
			deletedObjectCount += int64(len(ids))
			return nil
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return deletedObjectCount, nil
			}
			return deletedObjectCount, Error.Wrap(err)
		}
		if len(deleteSegments) == 0 {
			return deletedObjectCount, nil
		}

		if opts.DeletePieces != nil {
			err = opts.DeletePieces(ctx, deleteSegments)
			if err != nil {
				return deletedObjectCount, Error.Wrap(err)
			}
		}
	}
}
