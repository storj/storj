// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// DeleteObjectExactVersion contains arguments necessary for deleting an exact version of object.
type DeleteObjectExactVersion struct {
	Version Version
	ObjectLocation
}

// Verify delete object fields.
func (obj *DeleteObjectExactVersion) Verify() error {
	if err := obj.ObjectLocation.Verify(); err != nil {
		return err
	}
	if obj.Version <= 0 {
		return ErrInvalidRequest.New("Version invalid: %v", obj.Version)
	}
	return nil
}

// DeleteObjectResult result of deleting object.
type DeleteObjectResult struct {
	Objects  []Object
	Segments []DeletedSegmentInfo
}

// DeletedSegmentInfo info about deleted segment.
type DeletedSegmentInfo struct {
	RootPieceID storj.PieceID
	Pieces      Pieces
}

type deletedObjectInfo struct {
	Object
	Segments []deletedRemoteSegmentInfo

	// while deletion we are trying to find if deleted object have a copy
	// and if we need new ancestor to replace it. If we find a copy that
	// can be new ancestor we are keeping its stream id in this field.
	PromotedAncestor *uuid.UUID
}

type deletedRemoteSegmentInfo struct {
	Position    SegmentPosition
	RootPieceID storj.PieceID
	Pieces      Pieces
	RepairedAt  *time.Time
}

// DeleteObjectAnyStatusAllVersions contains arguments necessary for deleting all object versions.
type DeleteObjectAnyStatusAllVersions struct {
	ObjectLocation
}

// DeleteObjectsAllVersions contains arguments necessary for deleting all versions of multiple objects from the same bucket.
type DeleteObjectsAllVersions struct {
	Locations []ObjectLocation
}

// Verify delete objects fields.
func (delete *DeleteObjectsAllVersions) Verify() error {
	if len(delete.Locations) == 0 {
		return nil
	}

	if len(delete.Locations) > 1000 {
		return ErrInvalidRequest.New("cannot delete more than 1000 objects in a single request")
	}

	var errGroup errs.Group
	for _, location := range delete.Locations {
		errGroup.Add(location.Verify())
	}

	err := errGroup.Err()
	if err != nil {
		return err
	}

	// Verify if all locations are in the same bucket
	first := delete.Locations[0]
	for _, item := range delete.Locations[1:] {
		if first.ProjectID != item.ProjectID || first.BucketName != item.BucketName {
			return ErrInvalidRequest.New("all objects must be in the same bucket")
		}
	}

	return nil
}

var deleteObjectExactVersionWithoutCopyFeatureSQL = `
WITH deleted_objects AS (
	DELETE FROM objects
	WHERE
		project_id   = $1 AND
		bucket_name  = $2 AND
		object_key   = $3 AND
		version      = $4
	RETURNING
		version, stream_id,
		created_at, expires_at,
		status, segment_count,
		encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
		total_plain_size, total_encrypted_size, fixed_segment_size,
		encryption
), deleted_segments AS (
	DELETE FROM segments
	WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING segments.stream_id, segments.root_piece_id, segments.remote_alias_pieces
)
SELECT
	deleted_objects.version, deleted_objects.stream_id,
	deleted_objects.created_at, deleted_objects.expires_at,
	deleted_objects.status, deleted_objects.segment_count,
	deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
	deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
	deleted_objects.encryption,
	deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces
FROM deleted_objects
LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id`

var deleteObjectLastCommittedWithoutCopyFeatureSQL = `
WITH deleted_objects AS (
	DELETE FROM objects
	WHERE
		project_id   = $1 AND
		bucket_name  = $2 AND
		object_key   = $3 AND
		version IN (SELECT version FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			status       = ` + committedStatus + ` AND
			(expires_at IS NULL OR expires_at > now())
			ORDER BY version DESC
		)
	RETURNING
		version, stream_id,
		created_at, expires_at,
		status, segment_count,
		encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
		total_plain_size, total_encrypted_size, fixed_segment_size,
		encryption
), deleted_segments AS (
	DELETE FROM segments
	WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING segments.stream_id, segments.root_piece_id, segments.remote_alias_pieces
)
SELECT
	deleted_objects.version, deleted_objects.stream_id,
	deleted_objects.created_at, deleted_objects.expires_at,
	deleted_objects.status, deleted_objects.segment_count,
	deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
	deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
	deleted_objects.encryption,
	deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces
FROM deleted_objects
LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id`

// TODO: remove comments with regex.
var deleteBucketObjectsWithCopyFeatureSQL = `
WITH deleted_objects AS (
	%s
	RETURNING
		stream_id
		-- extra properties only returned when deleting single object
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
	-- select only one child to promote per ancestor
	SELECT DISTINCT ON (segment_copies.ancestor_stream_id)
		segment_copies.stream_id AS new_ancestor_stream_id,
		segment_copies.ancestor_stream_id AS deleted_stream_id
	FROM segment_copies
	-- select children about to lose their ancestor
	-- this is not a WHERE clause because that caused a full table scan in CockroachDB
	INNER JOIN deleted_objects
		ON deleted_objects.stream_id = segment_copies.ancestor_stream_id
	-- don't select children which will be removed themselves
	WHERE segment_copies.stream_id NOT IN (
		SELECT stream_id
		FROM deleted_objects
	)
)
SELECT
	deleted_objects.stream_id,
	deleted_segments.position,
	deleted_segments.root_piece_id,
	-- piece to remove from storagenodes or link to new ancestor
	deleted_segments.remote_alias_pieces,
	-- if set, caller needs to promote this stream_id to new ancestor or else object contents will be lost
	promoted_ancestors.new_ancestor_stream_id
	-- extra properties only returned when deleting single object
	%s
FROM deleted_objects
LEFT JOIN deleted_segments
	ON deleted_objects.stream_id = deleted_segments.stream_id
LEFT JOIN promoted_ancestors
	ON deleted_objects.stream_id = promoted_ancestors.deleted_stream_id
ORDER BY stream_id
`

var deleteObjectExactVersionSubSQL = `
DELETE FROM objects
WHERE
	project_id   = $1 AND
	bucket_name  = $2 AND
	object_key   = $3 AND
	version      = $4
`

var deleteObjectLastCommittedSubSQL = `
DELETE FROM objects
WHERE
	project_id   = $1 AND
	bucket_name  = $2 AND
	object_key   = $3 AND
	version IN (SELECT version FROM objects WHERE
		project_id   = $1 AND
		bucket_name  = $2 AND
		object_key   = $3 AND
		status       = ` + committedStatus + ` AND
		(expires_at IS NULL OR expires_at > now())
		ORDER BY version DESC
	)
`

var deleteObjectExactVersionWithCopyFeatureSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectExactVersionSubSQL,
	`,version,
		created_at,
		expires_at,
		status,
		segment_count,
		encrypted_metadata_nonce,
		encrypted_metadata,
		encrypted_metadata_encrypted_key,
		total_plain_size,
		total_encrypted_size,
		fixed_segment_size,
		encryption`,
	`,deleted_objects.version,
		deleted_objects.created_at,
		deleted_objects.expires_at,
		deleted_objects.status,
		deleted_objects.segment_count,
		deleted_objects.encrypted_metadata_nonce,
		deleted_objects.encrypted_metadata,
		deleted_objects.encrypted_metadata_encrypted_key,
		deleted_objects.total_plain_size,
		deleted_objects.total_encrypted_size,
		deleted_objects.fixed_segment_size,
		deleted_objects.encryption,
		deleted_segments.repaired_at`,
)

var deleteObjectLastCommittedWithCopyFeatureSQL = fmt.Sprintf(
	deleteBucketObjectsWithCopyFeatureSQL,
	deleteObjectLastCommittedSubSQL,
	`,version,
		created_at,
		expires_at,
		status,
		segment_count,
		encrypted_metadata_nonce,
		encrypted_metadata,
		encrypted_metadata_encrypted_key,
		total_plain_size,
		total_encrypted_size,
		fixed_segment_size,
		encryption`,
	`,deleted_objects.version,
		deleted_objects.created_at,
		deleted_objects.expires_at,
		deleted_objects.status,
		deleted_objects.segment_count,
		deleted_objects.encrypted_metadata_nonce,
		deleted_objects.encrypted_metadata,
		deleted_objects.encrypted_metadata_encrypted_key,
		deleted_objects.total_plain_size,
		deleted_objects.total_encrypted_size,
		deleted_objects.fixed_segment_size,
		deleted_objects.encryption,
		deleted_segments.repaired_at`,
)

var deleteFromSegmentCopies = `
	DELETE FROM segment_copies WHERE segment_copies.stream_id = $1
`

var updateSegmentsWithAncestor = `
	WITH update_segment_copies AS (
		UPDATE segment_copies
		SET ancestor_stream_id = $2
		WHERE ancestor_stream_id = $1
		RETURNING false
	)
	UPDATE segments
	SET
		remote_alias_pieces = P.remote_alias_pieces,
		repaired_at         = P.repaired_at
	FROM (SELECT UNNEST($3::INT8[]), UNNEST($4::BYTEA[]), UNNEST($5::timestamptz[]))
		as P(position, remote_alias_pieces, repaired_at)
	WHERE
		segments.stream_id = $2 AND
		segments.position = P.position
`

// DeleteObjectExactVersion deletes an exact object version.
//
// Result will contain only those segments which needs to be deleted
// from storage nodes. If object is an ancestor for copied object its
// segments pieces cannot be deleted because copy still needs it.
func (db *DB) DeleteObjectExactVersion(
	ctx context.Context, opts DeleteObjectExactVersion,
) (result DeleteObjectResult, err error) {
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		result, err = db.deleteObjectExactVersion(ctx, opts, tx)
		if err != nil {
			return err
		}
		return nil
	})

	return result, err
}

// implementation of DB.DeleteObjectExactVersion for re-use internally in metabase package.
func (db *DB) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion, tx tagsql.Tx) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	if db.config.ServerSideCopy {
		objects, err := db.deleteObjectExactVersionServerSideCopy(ctx, opts, tx)
		if err != nil {
			return DeleteObjectResult{}, err
		}

		for _, object := range objects {
			result.Objects = append(result.Objects, object.Object)

			// if object is ancestor for copied object we cannot delete its
			// segments pieces from storage nodes so we are not returning it
			// as an object deletion result
			if object.PromotedAncestor != nil {
				continue
			}
			for _, segment := range object.Segments {
				result.Segments = append(result.Segments, DeletedSegmentInfo{
					RootPieceID: segment.RootPieceID,
					Pieces:      segment.Pieces,
				})
			}
		}
	} else {
		err = withRows(
			tx.QueryContext(ctx, deleteObjectExactVersionWithoutCopyFeatureSQL,
				opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version),
		)(func(rows tagsql.Rows) error {
			result.Objects, result.Segments, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
			return err
		})
	}
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
}

func (db *DB) deleteObjectExactVersionServerSideCopy(ctx context.Context, opts DeleteObjectExactVersion, tx tagsql.Tx) (objects []deletedObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(
		tx.QueryContext(ctx, deleteObjectExactVersionWithCopyFeatureSQL, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version),
	)(func(rows tagsql.Rows) error {
		objects, err = db.scanObjectDeletionServerSideCopy(ctx, opts.ObjectLocation, rows)
		return err
	})
	if err != nil {
		return nil, err
	}

	err = db.promoteNewAncestors(ctx, tx, objects)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (db *DB) promoteNewAncestors(ctx context.Context, tx tagsql.Tx, objects []deletedObjectInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, object := range objects {
		if object.PromotedAncestor == nil {
			continue
		}

		positions := make([]int64, len(object.Segments))
		remoteAliasesPieces := make([][]byte, len(object.Segments))
		repairedAts := make([]*time.Time, len(object.Segments))

		for i, segment := range object.Segments {
			positions[i] = int64(segment.Position.Encode())

			aliases, err := db.aliasCache.EnsurePiecesToAliases(ctx, segment.Pieces)
			if err != nil {
				return err
			}

			aliasesBytes, err := aliases.Bytes()
			if err != nil {
				return err
			}
			remoteAliasesPieces[i] = aliasesBytes
			repairedAts[i] = segment.RepairedAt
		}

		result, err := tx.ExecContext(ctx, deleteFromSegmentCopies, *object.PromotedAncestor)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected != 1 {
			return errs.New("new ancestor was not deleted from segment copies")
		}

		result, err = tx.ExecContext(ctx, updateSegmentsWithAncestor,
			object.StreamID, *object.PromotedAncestor, pgutil.Int8Array(positions),
			pgutil.ByteaArray(remoteAliasesPieces), pgutil.NullTimestampTZArray(repairedAts))
		if err != nil {
			return err
		}

		affected, err = result.RowsAffected()
		if err != nil {
			return err
		}

		if affected != int64(len(object.Segments)) {
			return errs.New("not all new ancestor segments were update: got %d want %d", affected, len(object.Segments))
		}
	}
	return nil
}

// DeletePendingObject contains arguments necessary for deleting a pending object.
type DeletePendingObject struct {
	ObjectStream
}

// Verify verifies delete pending object fields validity.
func (opts *DeletePendingObject) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}
	return nil
}

// DeletePendingObject deletes a pending object with specified version and streamID.
func (db *DB) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = withRows(db.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE
					project_id   = $1 AND
					bucket_name  = $2 AND
					object_key   = $3 AND
					version      = $4 AND
					stream_id    = $5 AND
					status       = `+pendingStatus+`
				RETURNING
					version, stream_id,
					created_at, expires_at,
					status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					total_plain_size, total_encrypted_size, fixed_segment_size,
					encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id,segments.root_piece_id, segments.remote_alias_pieces
			)
			SELECT
				deleted_objects.version, deleted_objects.stream_id,
				deleted_objects.created_at, deleted_objects.expires_at,
				deleted_objects.status, deleted_objects.segment_count,
				deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
				deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
				deleted_objects.encryption,
				deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces
			FROM deleted_objects
			LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID))(func(rows tagsql.Rows) error {
		result.Objects, result.Segments, err = db.scanObjectDeletion(ctx, opts.Location(), rows)
		return err
	})

	if err != nil {
		return DeleteObjectResult{}, err
	}

	if len(result.Objects) == 0 {
		return DeleteObjectResult{}, ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
}

// DeleteObjectAnyStatusAllVersions deletes all object versions.
func (db *DB) DeleteObjectAnyStatusAllVersions(ctx context.Context, opts DeleteObjectAnyStatusAllVersions) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.config.ServerSideCopy {
		return DeleteObjectResult{}, errs.New("method cannot be used when server-side copy is enabled")
	}

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = withRows(db.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3
				RETURNING
					version, stream_id,
					created_at, expires_at,
					status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					total_plain_size, total_encrypted_size, fixed_segment_size,
					encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id,segments.root_piece_id, segments.remote_alias_pieces
			)
			SELECT
				deleted_objects.version, deleted_objects.stream_id,
				deleted_objects.created_at, deleted_objects.expires_at,
				deleted_objects.status, deleted_objects.segment_count,
				deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
				deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
				deleted_objects.encryption,
				deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces
			FROM deleted_objects
			LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey))(func(rows tagsql.Rows) error {
		result.Objects, result.Segments, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
		return err
	})

	if err != nil {
		return DeleteObjectResult{}, err
	}

	if len(result.Objects) == 0 {
		return DeleteObjectResult{}, ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
}

// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
func (db *DB) DeleteObjectsAllVersions(ctx context.Context, opts DeleteObjectsAllVersions) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.config.ServerSideCopy {
		return DeleteObjectResult{}, errs.New("method cannot be used when server-side copy is enabled")
	}

	if len(opts.Locations) == 0 {
		// nothing to delete, no error
		return DeleteObjectResult{}, nil
	}

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	// It is aleady verified that all object locations are in the same bucket
	projectID := opts.Locations[0].ProjectID
	bucketName := opts.Locations[0].BucketName

	objectKeys := make([][]byte, len(opts.Locations))
	for i := range opts.Locations {
		objectKeys[i] = []byte(opts.Locations[i].ObjectKey)
	}

	// Sorting the object keys just in case.
	// TODO: Check if this is really necessary for the SQL query.
	sort.Slice(objectKeys, func(i, j int) bool {
		return bytes.Compare(objectKeys[i], objectKeys[j]) < 0
	})
	err = withRows(db.db.QueryContext(ctx, `
				WITH deleted_objects AS (
					DELETE FROM objects
					WHERE
					project_id   = $1 AND
					bucket_name  = $2 AND
					object_key   = ANY ($3) AND
					status       = `+committedStatus+`
					RETURNING
						project_id, bucket_name,
						object_key, version, stream_id,
						created_at, expires_at,
						status, segment_count,
						encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
						total_plain_size, total_encrypted_size, fixed_segment_size,
						encryption
				), deleted_segments AS (
					DELETE FROM segments
					WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
					RETURNING segments.stream_id,segments.root_piece_id, segments.remote_alias_pieces
				)
				SELECT
					deleted_objects.project_id, deleted_objects.bucket_name,
					deleted_objects.object_key,deleted_objects.version, deleted_objects.stream_id,
					deleted_objects.created_at, deleted_objects.expires_at,
					deleted_objects.status, deleted_objects.segment_count,
					deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
					deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
					deleted_objects.encryption,
					deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces
				FROM deleted_objects
				LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id
			`, projectID, []byte(bucketName), pgutil.ByteaArray(objectKeys)))(func(rows tagsql.Rows) error {
		result.Objects, result.Segments, err = db.scanMultipleObjectsDeletion(ctx, rows)
		return err
	})

	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
}

func (db *DB) scanObjectDeletionServerSideCopy(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (result []deletedObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	defer func() { err = errs.Combine(err, rows.Close()) }()

	result = make([]deletedObjectInfo, 0, 10)

	var rootPieceID *storj.PieceID
	// for object without segments we can get position = NULL
	var segmentPosition *SegmentPosition
	var object deletedObjectInfo
	var segment deletedRemoteSegmentInfo
	var aliasPieces AliasPieces

	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(
			// shared properties between deleteObject and deleteBucketObjects functionality
			&object.StreamID,
			&segmentPosition,
			&rootPieceID,
			&aliasPieces,
			&object.PromotedAncestor,
			// properties only for deleteObject functionality
			&object.Version,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
			&segment.RepairedAt,
		)
		if err != nil {
			return nil, Error.New("unable to delete object: %w", err)
		}
		if len(result) == 0 || result[len(result)-1].StreamID != object.StreamID {
			result = append(result, object)
		}

		if rootPieceID != nil {
			if segmentPosition != nil {
				segment.Position = *segmentPosition
			}

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

func (db *DB) scanObjectDeletion(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (objects []Object, segments []DeletedSegmentInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	defer func() { err = errs.Combine(err, rows.Close()) }()

	objects = make([]Object, 0, 10)
	segments = make([]DeletedSegmentInfo, 0, 10)

	var rootPieceID *storj.PieceID
	var object Object
	var segment DeletedSegmentInfo
	var aliasPieces AliasPieces

	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption}, &rootPieceID, &aliasPieces,
		)
		if err != nil {
			return nil, nil, Error.New("unable to delete object: %w", err)
		}
		if len(objects) == 0 || objects[len(objects)-1].StreamID != object.StreamID {
			objects = append(objects, object)
		}

		if rootPieceID != nil {
			segment.RootPieceID = *rootPieceID
			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				return nil, nil, Error.Wrap(err)
			}
			if len(segment.Pieces) > 0 {
				segments = append(segments, segment)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, Error.New("unable to delete object: %w", err)
	}

	if len(segments) == 0 {
		return objects, nil, nil
	}
	return objects, segments, nil
}

func (db *DB) scanMultipleObjectsDeletion(ctx context.Context, rows tagsql.Rows) (objects []Object, segments []DeletedSegmentInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	defer func() { err = errs.Combine(err, rows.Close()) }()

	objects = make([]Object, 0, 10)
	segments = make([]DeletedSegmentInfo, 0, 10)

	var rootPieceID *storj.PieceID
	var object Object
	var segment DeletedSegmentInfo
	var aliasPieces AliasPieces

	for rows.Next() {
		err = rows.Scan(&object.ProjectID, &object.BucketName,
			&object.ObjectKey, &object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption}, &rootPieceID, &aliasPieces)
		if err != nil {
			return nil, nil, Error.New("unable to delete object: %w", err)
		}

		if len(objects) == 0 || objects[len(objects)-1].StreamID != object.StreamID {
			objects = append(objects, object)
		}
		if rootPieceID != nil {
			segment.RootPieceID = *rootPieceID
			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				return nil, nil, Error.Wrap(err)
			}
			if len(segment.Pieces) > 0 {
				segments = append(segments, segment)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, Error.New("unable to delete object: %w", err)
	}

	if len(objects) == 0 {
		objects = nil
	}
	if len(segments) == 0 {
		return objects, nil, nil
	}

	return objects, segments, nil
}

// DeleteObjectLastCommitted contains arguments necessary for deleting last committed version of object.
type DeleteObjectLastCommitted struct {
	ObjectLocation
}

// Verify delete object last committed fields.
func (obj *DeleteObjectLastCommitted) Verify() error {
	return obj.ObjectLocation.Verify()
}

// DeleteObjectLastCommitted deletes an object last committed version.
//
// Result will contain only those segments which needs to be deleted
// from storage nodes. If object is an ancestor for copied object its
// segments pieces cannot be deleted because copy still needs it.
func (db *DB) DeleteObjectLastCommitted(
	ctx context.Context, opts DeleteObjectLastCommitted,
) (result DeleteObjectResult, err error) {
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		result, err = db.deleteObjectLastCommitted(ctx, opts, tx)
		if err != nil {
			return err
		}
		return nil
	})
	return result, err
}

// implementation of DB.DeleteObjectLastCommitted for re-use internally in metabase package.
func (db *DB) deleteObjectLastCommitted(ctx context.Context, opts DeleteObjectLastCommitted, tx tagsql.Tx) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	if db.config.ServerSideCopy {
		objects, err := db.deleteObjectLastCommittedServerSideCopy(ctx, opts, tx)
		if err != nil {
			return DeleteObjectResult{}, err
		}

		for _, object := range objects {
			result.Objects = append(result.Objects, object.Object)

			// if object is ancestor for copied object we cannot delete its
			// segments pieces from storage nodes so we are not returning it
			// as an object deletion result
			if object.PromotedAncestor != nil {
				continue
			}
			for _, segment := range object.Segments {
				result.Segments = append(result.Segments, DeletedSegmentInfo{
					RootPieceID: segment.RootPieceID,
					Pieces:      segment.Pieces,
				})
			}
		}
	} else {
		err = withRows(
			tx.QueryContext(ctx, deleteObjectLastCommittedWithoutCopyFeatureSQL,
				opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey),
		)(func(rows tagsql.Rows) error {
			result.Objects, result.Segments, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
			return err
		})
	}
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
}

func (db *DB) deleteObjectLastCommittedServerSideCopy(ctx context.Context, opts DeleteObjectLastCommitted, tx tagsql.Tx) (objects []deletedObjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(
		tx.QueryContext(ctx, deleteObjectLastCommittedWithCopyFeatureSQL, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey),
	)(func(rows tagsql.Rows) error {
		objects, err = db.scanObjectDeletionServerSideCopy(ctx, opts.ObjectLocation, rows)
		return err
	})
	if err != nil {
		return nil, err
	}

	err = db.promoteNewAncestors(ctx, tx, objects)
	if err != nil {
		return nil, err
	}

	return objects, nil
}
