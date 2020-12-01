// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/dbutil/txutil"
	"storj.io/storj/private/tagsql"
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

// DeleteObjectAllVersions contains arguments necessary for deleting all object versions.
type DeleteObjectAllVersions struct {
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

	// TODO: make this limit configurable
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

// DeleteObjectLatestVersion contains arguments necessary for deleting latest object version.
type DeleteObjectLatestVersion struct {
	ObjectLocation
}

// DeleteObjectExactVersion deletes an exact object version.
func (db *DB) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		rows, err := tx.Query(ctx, `
			DELETE FROM objects
			WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				version      = $4 AND
				status       = `+committedStatus+`
			RETURNING
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption;
		`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
			}
			return Error.New("unable to delete object: %w", err)
		}

		result.Objects, err = scanObjectDeletion(opts.ObjectLocation, rows)
		if err != nil {
			return err
		}

		if len(result.Objects) == 0 {
			return storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
		}

		segmentInfos, err := deleteSegments(ctx, tx, result.Objects)
		if err != nil {
			return err
		}

		if len(segmentInfos) != 0 {
			result.Segments = segmentInfos
		}
		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeletePendingObject contains arguments necessary for deleting a pending object.
type DeletePendingObject struct {
	ObjectLocation
	Version
	StreamID uuid.UUID
}

// Verify verifies delete pending object fields validity.
func (opts *DeletePendingObject) Verify() error {
	if err := opts.ObjectLocation.Verify(); err != nil {
		return err
	}

	if opts.Version <= 0 {
		return ErrInvalidRequest.New("Version invalid: %v", opts.Version)
	}

	if opts.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// DeletePendingObject deletes a pending object with specified version and streamID.
func (db *DB) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		rows, err := tx.Query(ctx, `
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
				encryption;
		`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
			}
			return Error.New("unable to delete object: %w", err)
		}

		result.Objects, err = scanObjectDeletion(opts.ObjectLocation, rows)
		if err != nil {
			return err
		}

		if len(result.Objects) == 0 {
			return storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
		}

		segmentInfos, err := deleteSegments(ctx, tx, result.Objects)
		if err != nil {
			return err
		}

		if len(segmentInfos) != 0 {
			result.Segments = segmentInfos
		}
		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeleteObjectLatestVersion deletes latest object version.
func (db *DB) DeleteObjectLatestVersion(ctx context.Context, opts DeleteObjectLatestVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		// TODO different sql for Postgres and CockroachDB
		// version ONLY for cockroachdb
		// Postgres doesn't support ORDER BY and LIMIT in DELETE
		// rows, err = tx.Query(ctx, `
		// DELETE FROM objects
		// WHERE
		// 	project_id   = $1 AND
		// 	bucket_name  = $2 AND
		// 	object_key   = $3 AND
		// 	status       = 1
		// ORDER BY version DESC
		// LIMIT 1
		// RETURNING stream_id;
		// `, opts.ProjectID, opts.BucketName, opts.ObjectKey)

		// version for Postgres and Cockroachdb (but slow for Cockroachdb)
		rows, err := tx.Query(ctx, `
			DELETE FROM objects
			WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				version      = (SELECT version FROM objects WHERE
					project_id   = $1 AND
					bucket_name  = $2 AND
					object_key   = $3 AND
					status       = `+committedStatus+`
					ORDER BY version DESC LIMIT 1
				) AND
				status       = `+committedStatus+`
			RETURNING
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption;
		`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
			}
			return Error.New("unable to delete object: %w", err)
		}

		result.Objects, err = scanObjectDeletion(opts.ObjectLocation, rows)
		if err != nil {
			return err
		}

		if len(result.Objects) == 0 {
			return storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
		}

		segmentInfos, err := deleteSegments(ctx, tx, result.Objects)
		if err != nil {
			return err
		}

		if len(segmentInfos) != 0 {
			result.Segments = segmentInfos
		}
		return nil
	})

	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeleteObjectAllVersions deletes all object versions.
func (db *DB) DeleteObjectAllVersions(ctx context.Context, opts DeleteObjectAllVersions) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		rows, err := tx.Query(ctx, `
			DELETE FROM objects
			WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				status       = `+committedStatus+`
			RETURNING
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption;
		`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
			}
			return Error.New("unable to delete object: %w", err)
		}

		result.Objects, err = scanObjectDeletion(opts.ObjectLocation, rows)
		if err != nil {
			return err
		}

		if len(result.Objects) == 0 {
			return storj.ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
		}

		segmentInfos, err := deleteSegments(ctx, tx, result.Objects)
		if err != nil {
			return err
		}

		if len(segmentInfos) != 0 {
			result.Segments = segmentInfos
		}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
func (db *DB) DeleteObjectsAllVersions(ctx context.Context, opts DeleteObjectsAllVersions) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(opts.Locations) == 0 {
		// nothing to delete, no error
		return DeleteObjectResult{}, nil
	}

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
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

		rows, err := tx.Query(ctx, `
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
					encryption;
		`, projectID, bucketName, pgutil.ByteaArray(objectKeys))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
			}
			return Error.New("unable to delete object: %w", err)
		}

		result.Objects, err = scanMultipleObjectsDeletion(rows)
		if err != nil {
			return err
		}

		if len(result.Objects) == 0 {
			// nothing was delete, no error
			return nil
		}

		segmentInfos, err := deleteSegments(ctx, tx, result.Objects)
		if err != nil {
			return err
		}

		if len(segmentInfos) != 0 {
			result.Segments = segmentInfos
		}
		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

func scanObjectDeletion(location ObjectLocation, rows tagsql.Rows) (objects []Object, err error) {
	defer func() { err = errs.Combine(err, rows.Close()) }()

	objects = make([]Object, 0, 10)
	for rows.Next() {
		var object Object
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption})
		if err != nil {
			return nil, Error.New("unable to delete object: %w", err)
		}

		objects = append(objects, object)
	}

	if err := rows.Err(); err != nil {
		return nil, Error.New("unable to delete object: %w", err)
	}

	return objects, nil
}

func scanMultipleObjectsDeletion(rows tagsql.Rows) (objects []Object, err error) {
	defer func() { err = errs.Combine(err, rows.Close()) }()

	objects = make([]Object, 0, 10)
	for rows.Next() {
		var object Object
		err = rows.Scan(&object.ProjectID, &object.BucketName,
			&object.ObjectKey, &object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption})
		if err != nil {
			return nil, Error.New("unable to delete object: %w", err)
		}
		objects = append(objects, object)
	}

	if err := rows.Err(); err != nil {
		return nil, Error.New("unable to delete object: %w", err)
	}

	if len(objects) == 0 {
		objects = nil
	}

	return objects, nil
}

func deleteSegments(ctx context.Context, tx tagsql.Tx, objects []Object) (_ []DeletedSegmentInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO we need to figure out how integrate this with piece deletion code
	// one issue is that with this approach we need to return all pieces SN ids at once

	streamIDs := make([][]byte, len(objects))
	for i := range objects {
		streamIDs[i] = objects[i].StreamID[:]
	}

	// Sorting the stream IDs just in case.
	// TODO: Check if this is really necessary for the SQL query.
	sort.Slice(streamIDs, func(i, j int) bool {
		return bytes.Compare(streamIDs[i], streamIDs[j]) < 0
	})

	segmentsRows, err := tx.Query(ctx, `
			DELETE FROM segments
			WHERE stream_id = ANY ($1)
			RETURNING root_piece_id, remote_pieces;
		`, pgutil.ByteaArray(streamIDs))
	if err != nil {
		return []DeletedSegmentInfo{}, Error.New("unable to delete object: %w", err)
	}
	defer func() { err = errs.Combine(err, segmentsRows.Close()) }()

	infos := make([]DeletedSegmentInfo, 0, len(objects))
	for segmentsRows.Next() {
		var segmentInfo DeletedSegmentInfo
		err = segmentsRows.Scan(&segmentInfo.RootPieceID, &segmentInfo.Pieces)
		if err != nil {
			return []DeletedSegmentInfo{}, Error.New("unable to delete object: %w", err)
		}

		if len(segmentInfo.Pieces) != 0 {
			infos = append(infos, segmentInfo)
		}
	}
	if err := segmentsRows.Err(); err != nil {
		return []DeletedSegmentInfo{}, Error.New("unable to delete object: %w", err)
	}

	return infos, nil
}
