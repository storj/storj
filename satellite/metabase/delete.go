// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
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
	RETURNING segments.stream_id,segments.root_piece_id, segments.remote_alias_pieces
)
SELECT
	deleted_objects.version, deleted_objects.stream_id,
	deleted_objects.created_at, deleted_objects.expires_at,
	deleted_objects.status, deleted_objects.segment_count,
	deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
	deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
	deleted_objects.encryption,
	deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces,
	NULL
FROM deleted_objects
LEFT JOIN deleted_segments ON deleted_objects.stream_id = deleted_segments.stream_id`

// There are 3 scenarios:
// 1) If the object to be removed is singular, it should simply remove the object and its segments
// 2) If the object is a copy, it should also remove the entry from segment_copies
// 3) If the object has copies of its own, a new ancestor should be promoted,
//    removed from segment_copies, and the other copies should point to the new ancestor
var deleteObjectExactVersionWithCopyFeatureSQL = `
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
),
deleted_segments AS (
	DELETE FROM segments
	WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING
		segments.stream_id,
		segments.root_piece_id,
		segments.remote_alias_pieces,
		segments.plain_size,
		segments.encrypted_size,
		segments.position,
		segments.inline_data,
		segments.repaired_at
),
deleted_copies AS (
	DELETE FROM segment_copies
	WHERE segment_copies.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
	RETURNING segment_copies.stream_id
),
-- lowest stream_id becomes new ancestor
promoted_ancestor AS (
	DELETE FROM segment_copies
	WHERE segment_copies.stream_id = (
		SELECT stream_id
		FROM segment_copies
		WHERE segment_copies.ancestor_stream_id = (
			SELECT stream_id
			FROM deleted_objects
			ORDER BY stream_id
			LIMIT 1
		)
		ORDER BY stream_id
		LIMIT 1
	)
	RETURNING
		stream_id
),
update_other_copies_with_new_ancestor AS (
	UPDATE segment_copies
	SET ancestor_stream_id = (SELECT stream_id FROM promoted_ancestor)
	WHERE segment_copies.ancestor_stream_id IN (SELECT stream_id FROM deleted_objects)
		-- bit weird condition but this seems necessary to prevent it from updating a row which is being deleted
		AND segment_copies.stream_id != (SELECT stream_id FROM promoted_ancestor)
	RETURNING segment_copies.stream_id
),
update_copy_promoted_to_ancestors AS (
	UPDATE segments AS promoted_segments
	SET
		remote_alias_pieces = deleted_segments.remote_alias_pieces,
		inline_data = deleted_segments.inline_data,
		plain_size = deleted_segments.plain_size,
		encrypted_size = deleted_segments.encrypted_size,
		repaired_at = deleted_segments.repaired_at
	FROM deleted_segments
	-- i suppose this only works when there is 1 stream_id being deleted
	WHERE
		promoted_segments.stream_id IN (SELECT stream_id FROM promoted_ancestor)
		AND promoted_segments.position = deleted_segments.position
	RETURNING deleted_segments.stream_id
)
SELECT
	deleted_objects.version, deleted_objects.stream_id,
	deleted_objects.created_at, deleted_objects.expires_at,
	deleted_objects.status, deleted_objects.segment_count,
	deleted_objects.encrypted_metadata_nonce, deleted_objects.encrypted_metadata, deleted_objects.encrypted_metadata_encrypted_key,
	deleted_objects.total_plain_size, deleted_objects.total_encrypted_size, deleted_objects.fixed_segment_size,
	deleted_objects.encryption,
	deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces,
	(SELECT stream_id FROM promoted_ancestor)
FROM deleted_objects
LEFT JOIN deleted_segments
	ON deleted_objects.stream_id = deleted_segments.stream_id`

// DeleteObjectExactVersion deletes an exact object version.
func (db *DB) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	deleteSQL := deleteObjectExactVersionWithoutCopyFeatureSQL
	if db.config.ServerSideCopy {
		deleteSQL = deleteObjectExactVersionWithCopyFeatureSQL
	}

	err = withRows(
		db.db.QueryContext(ctx, deleteSQL, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version),
	)(func(rows tagsql.Rows) error {
		result.Objects, result.Segments, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
		return err
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Objects))
	mon.Meter("segment_delete").Mark(len(result.Segments))

	return result, nil
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
				deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces,
				NULL
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
		return DeleteObjectResult{}, storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
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
				deleted_segments.root_piece_id, deleted_segments.remote_alias_pieces,
				NULL
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
		return DeleteObjectResult{}, storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
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
		var promotedAncestor *uuid.UUID
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption}, &rootPieceID, &aliasPieces,
			&promotedAncestor,
		)
		if err != nil {
			return nil, nil, Error.New("unable to delete object: %w", err)
		}
		if len(objects) == 0 || objects[len(objects)-1].StreamID != object.StreamID {
			objects = append(objects, object)
		}
		// not nil promotedAncestor means that while delete new ancestor was promoted and
		// we should not delete pieces because we had copies and now one copy become ancestor
		if rootPieceID != nil && promotedAncestor == nil {
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
