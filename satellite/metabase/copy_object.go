// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// BeginCopyObjectResult holds data needed to begin copy object.
type BeginCopyObjectResult BeginMoveCopyResults

// BeginCopyObject holds all data needed begin copy object method.
type BeginCopyObject struct {
	ObjectLocation

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out the copy would exceed a limit.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// BeginCopyObject collects all data needed to begin object copy procedure.
func (db *DB) BeginCopyObject(ctx context.Context, opts BeginCopyObject) (_ BeginCopyObjectResult, err error) {
	result, err := db.beginMoveCopyObject(ctx, opts.ObjectLocation, CopySegmentLimit, opts.VerifyLimits)
	if err != nil {
		return BeginCopyObjectResult{}, err
	}

	return BeginCopyObjectResult(result), nil
}

// FinishCopyObject holds all data needed to finish object copy.
type FinishCopyObject struct {
	ObjectStream
	NewBucket             string
	NewEncryptedObjectKey ObjectKey
	NewStreamID           uuid.UUID

	OverrideMetadata             bool
	NewEncryptedMetadata         []byte
	NewEncryptedMetadataKeyNonce storj.Nonce
	NewEncryptedMetadataKey      []byte

	NewSegmentKeys []EncryptedKeyAndNonce

	// If set, copy the object by duplicating the metadata and
	// remote_alias_pieces list, rather than using segment_copies.
	DuplicateMetadata bool

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out completing the copy would exceed a limit.
	// It will be called only once.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// Verify verifies metabase.FinishCopyObject data.
func (finishCopy FinishCopyObject) Verify() error {
	if err := finishCopy.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case len(finishCopy.NewBucket) == 0:
		return ErrInvalidRequest.New("NewBucket is missing")
	case finishCopy.NewStreamID.IsZero():
		return ErrInvalidRequest.New("NewStreamID is missing")
	case finishCopy.ObjectStream.StreamID == finishCopy.NewStreamID:
		return ErrInvalidRequest.New("StreamIDs are identical")
	case len(finishCopy.NewEncryptedObjectKey) == 0:
		return ErrInvalidRequest.New("NewEncryptedObjectKey is missing")
	}

	if finishCopy.OverrideMetadata {
		if finishCopy.NewEncryptedMetadata == nil && (!finishCopy.NewEncryptedMetadataKeyNonce.IsZero() || finishCopy.NewEncryptedMetadataKey != nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
		} else if finishCopy.NewEncryptedMetadata != nil && (finishCopy.NewEncryptedMetadataKeyNonce.IsZero() || finishCopy.NewEncryptedMetadataKey == nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
		}
	} else {
		switch {
		case finishCopy.NewEncryptedMetadataKeyNonce.IsZero() && len(finishCopy.NewEncryptedMetadataKey) != 0:
			return ErrInvalidRequest.New("EncryptedMetadataKeyNonce is missing")
		case len(finishCopy.NewEncryptedMetadataKey) == 0 && !finishCopy.NewEncryptedMetadataKeyNonce.IsZero():
			return ErrInvalidRequest.New("EncryptedMetadataKey is missing")
		}
	}

	return nil
}

// FinishCopyObject accepts new encryption keys for copied object and insert the corresponding new object ObjectKey and segments EncryptedKey.
// It returns the object at the destination location.
func (db *DB) FinishCopyObject(ctx context.Context, opts FinishCopyObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	newObject := Object{}
	var copyMetadata []byte

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		sourceObject, ancestorStreamID, objectAtDestination, nextAvailableVersion, err := getObjectAtCopySourceAndDestination(ctx, tx, opts)
		if err != nil {
			return err
		}

		if objectAtDestination != nil && objectAtDestination.StreamID == sourceObject.StreamID {
			newObject = sourceObject
			return nil
		}
		if opts.VerifyLimits != nil {
			err := opts.VerifyLimits(sourceObject.TotalEncryptedSize, int64(sourceObject.SegmentCount))
			if err != nil {
				return err
			}
		}

		if int(sourceObject.SegmentCount) != len(opts.NewSegmentKeys) {
			return ErrInvalidRequest.New("wrong number of segments keys received (received %d, need %d)", len(opts.NewSegmentKeys), sourceObject.SegmentCount)
		}

		var newSegments struct {
			Positions          []int64
			EncryptedKeys      [][]byte
			EncryptedKeyNonces [][]byte
		}

		for _, u := range opts.NewSegmentKeys {
			newSegments.EncryptedKeys = append(newSegments.EncryptedKeys, u.EncryptedKey)
			newSegments.EncryptedKeyNonces = append(newSegments.EncryptedKeyNonces, u.EncryptedKeyNonce)
			newSegments.Positions = append(newSegments.Positions, int64(u.Position.Encode()))
		}

		positions := make([]int64, sourceObject.SegmentCount)

		rootPieceIDs := make([][]byte, sourceObject.SegmentCount)

		expiresAts := make([]*time.Time, sourceObject.SegmentCount)
		encryptedSizes := make([]int32, sourceObject.SegmentCount)
		plainSizes := make([]int32, sourceObject.SegmentCount)
		plainOffsets := make([]int64, sourceObject.SegmentCount)
		inlineDatas := make([][]byte, sourceObject.SegmentCount)
		placementConstraints := make([]storj.PlacementConstraint, sourceObject.SegmentCount)
		remoteAliasPiecesLists := make([][]byte, sourceObject.SegmentCount)

		redundancySchemes := make([]int64, sourceObject.SegmentCount)

		if opts.DuplicateMetadata {
			err = withRows(db.db.QueryContext(ctx, `
				SELECT
					position,
					expires_at,
					root_piece_id,
					encrypted_size, plain_offset, plain_size,
					redundancy,
					remote_alias_pieces,
					placement,
					inline_data
				FROM segments
				WHERE stream_id = $1
				ORDER BY position ASC
				LIMIT  $2
			`, sourceObject.StreamID, sourceObject.SegmentCount))(func(rows tagsql.Rows) error {
				index := 0
				for rows.Next() {
					err := rows.Scan(
						&positions[index],
						&expiresAts[index],
						&rootPieceIDs[index],
						&encryptedSizes[index], &plainOffsets[index], &plainSizes[index],
						&redundancySchemes[index],
						&remoteAliasPiecesLists[index],
						&placementConstraints[index],
						&inlineDatas[index],
					)
					if err != nil {
						return err
					}
					index++
				}

				if err := rows.Err(); err != nil {
					return err
				}

				if index != int(sourceObject.SegmentCount) {
					return Error.New("could not load all of the segment information")
				}

				return nil
			})
		} else {
			err = withRows(db.db.QueryContext(ctx, `
				SELECT
					position,
					expires_at,
					root_piece_id,
					encrypted_size, plain_offset, plain_size,
					redundancy,
					inline_data
				FROM segments
				WHERE stream_id = $1
				ORDER BY position ASC
				LIMIT  $2
			`, sourceObject.StreamID, sourceObject.SegmentCount))(func(rows tagsql.Rows) error {
				index := 0
				for rows.Next() {
					err := rows.Scan(
						&positions[index],
						&expiresAts[index],
						&rootPieceIDs[index],
						&encryptedSizes[index], &plainOffsets[index], &plainSizes[index],
						&redundancySchemes[index],
						&inlineDatas[index],
					)
					if err != nil {
						return err
					}
					index++
				}

				if err := rows.Err(); err != nil {
					return err
				}

				if index != int(sourceObject.SegmentCount) {
					return Error.New("could not load all of the segment information")
				}

				return nil
			})
		}
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		onlyInlineSegments := true
		for index := range positions {
			if newSegments.Positions[index] != positions[index] {
				return Error.New("missing new segment keys for segment %d", positions[index])
			}
			if onlyInlineSegments && (encryptedSizes[index] > 0) && len(inlineDatas[index]) == 0 {
				onlyInlineSegments = false
			}
		}

		if opts.OverrideMetadata {
			copyMetadata = opts.NewEncryptedMetadata
		} else {
			copyMetadata = sourceObject.EncryptedMetadata
		}

		if objectAtDestination != nil {
			version := objectAtDestination.Version
			deletedObjects, err := db.deleteObjectExactVersion(
				ctx, DeleteObjectExactVersion{
					Version: version,
					ObjectLocation: ObjectLocation{
						ProjectID:  objectAtDestination.ProjectID,
						BucketName: objectAtDestination.BucketName,
						ObjectKey:  objectAtDestination.ObjectKey,
					},
				}, tx,
			)
			if err != nil {
				return Error.New("unable to delete existing object at copy destination: %w", err)
			}

			// The object at the destination was the ancestor!
			if ancestorStreamID == objectAtDestination.StreamID {
				if len(deletedObjects.Objects) == 0 {
					return Error.New("ancestor is gone, please retry operation")
				}
			}
		}

		// TODO we need to handle metadata correctly (copy from original object or replace)
		row := tx.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, status, segment_count,
				encryption,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				zombie_deletion_deadline
			) VALUES (
				$1, $2, $3, $4, $5,
				$6,`+committedStatus+`, $7,
				$8,
				$9, $10, $11,
				$12, $13, $14, null
			)
			RETURNING
				created_at`,
			opts.ProjectID, []byte(opts.NewBucket), opts.NewEncryptedObjectKey, nextAvailableVersion, opts.NewStreamID,
			sourceObject.ExpiresAt, sourceObject.SegmentCount,
			encryptionParameters{&sourceObject.Encryption},
			copyMetadata, opts.NewEncryptedMetadataKeyNonce, opts.NewEncryptedMetadataKey,
			sourceObject.TotalPlainSize, sourceObject.TotalEncryptedSize, sourceObject.FixedSegmentSize,
		)

		newObject = sourceObject
		newObject.Version = nextAvailableVersion

		err = row.Scan(&newObject.CreatedAt)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				encrypted_key_nonce, encrypted_key,
				root_piece_id,
				redundancy,
				encrypted_size, plain_offset, plain_size,
				remote_alias_pieces, placement,
				inline_data
			) SELECT
				$1, UNNEST($2::INT8[]), UNNEST($3::timestamptz[]),
				UNNEST($4::BYTEA[]), UNNEST($5::BYTEA[]),
				UNNEST($6::BYTEA[]),
				UNNEST($7::INT8[]),
				UNNEST($8::INT4[]), UNNEST($9::INT8[]),	UNNEST($10::INT4[]),
				UNNEST($11::BYTEA[]), UNNEST($12::INT2[]),
				UNNEST($13::BYTEA[])
		`, opts.NewStreamID, pgutil.Int8Array(newSegments.Positions), pgutil.NullTimestampTZArray(expiresAts),
			pgutil.ByteaArray(newSegments.EncryptedKeyNonces), pgutil.ByteaArray(newSegments.EncryptedKeys),
			pgutil.ByteaArray(rootPieceIDs),
			pgutil.Int8Array(redundancySchemes),
			pgutil.Int4Array(encryptedSizes), pgutil.Int8Array(plainOffsets), pgutil.Int4Array(plainSizes),
			pgutil.ByteaArray(remoteAliasPiecesLists), pgutil.PlacementConstraintArray(placementConstraints),
			pgutil.ByteaArray(inlineDatas),
		)
		if err != nil {
			return Error.New("unable to copy segments: %w", err)
		}

		if onlyInlineSegments {
			return nil
		}

		if !opts.DuplicateMetadata {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO segment_copies (
					stream_id, ancestor_stream_id
				) VALUES (
					$1, $2
				)
			`, opts.NewStreamID, ancestorStreamID)
			if err != nil {
				return Error.New("unable to copy object: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return Object{}, err
	}

	newObject.StreamID = opts.NewStreamID
	newObject.BucketName = opts.NewBucket
	newObject.ObjectKey = opts.NewEncryptedObjectKey
	newObject.EncryptedMetadata = copyMetadata
	newObject.EncryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
	if !opts.NewEncryptedMetadataKeyNonce.IsZero() {
		newObject.EncryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce[:]
	}

	mon.Meter("finish_copy_object").Mark(1)

	return newObject, nil
}

// Fetch the following in a single query:
// - object at copy source location (error if it's not there)
// - source ancestor stream id (if any)
// - next version available
// - object at copy destination location (if any).
func getObjectAtCopySourceAndDestination(
	ctx context.Context, tx tagsql.Tx, opts FinishCopyObject,
) (sourceObject Object, ancestorStreamID uuid.UUID, destinationObject *Object, nextAvailableVersion Version, err error) {
	defer mon.Task()(&ctx)(&err)

	var ancestorStreamIDBytes []byte
	var highestVersion Version

	sourceObject.ProjectID = opts.ProjectID
	sourceObject.BucketName = opts.BucketName
	sourceObject.ObjectKey = opts.ObjectKey
	sourceObject.Version = opts.Version
	sourceObject.Status = Committed

	// get objects at source and destination (if any)
	rows, err := tx.QueryContext(ctx, `
		WITH destination_current_versions AS (
			SELECT status, max(version) AS version
			FROM objects
			WHERE
				project_id  = $1 AND
				bucket_name = $5 AND
				object_key  = $6
			GROUP BY status
		)
		SELECT
			objects.stream_id,
			expires_at,
			segment_count,
			encrypted_metadata,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			segment_copies.ancestor_stream_id,
			0,
			coalesce((SELECT max(version) FROM destination_current_versions),0) AS highest_version
		FROM objects
		LEFT JOIN segment_copies ON objects.stream_id = segment_copies.stream_id
		WHERE
			project_id   = $1 AND
			bucket_name  = $3 AND
			object_key   = $4 AND
			version      = $2 AND
			status       = `+committedStatus+`
		UNION ALL
		SELECT
			stream_id,
			expires_at,
			segment_count,
			NULL,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			NULL,
			version,
			(SELECT max(version) FROM destination_current_versions) AS highest_version
		FROM objects
		WHERE
			project_id  = $1 AND
			bucket_name = $5 AND
			object_key  = $6 AND
			version     = (SELECT version FROM destination_current_versions
							WHERE status = `+committedStatus+`)`,
		sourceObject.ProjectID, sourceObject.Version,
		[]byte(sourceObject.BucketName), sourceObject.ObjectKey,
		opts.NewBucket, opts.NewEncryptedObjectKey)
	if err != nil {
		return Object{}, uuid.UUID{}, nil, 0, err
	}
	defer func() {
		err = errs.Combine(err, rows.Err(), rows.Close())
	}()

	if !rows.Next() {
		return Object{}, uuid.UUID{}, nil, 0, ErrObjectNotFound.New("source object not found")
	}

	err = rows.Scan(
		&sourceObject.StreamID,
		&sourceObject.ExpiresAt,
		&sourceObject.SegmentCount,
		&sourceObject.EncryptedMetadata,
		&sourceObject.TotalPlainSize, &sourceObject.TotalEncryptedSize, &sourceObject.FixedSegmentSize,
		encryptionParameters{&sourceObject.Encryption},
		&ancestorStreamIDBytes,
		&highestVersion,
		&highestVersion,
	)
	if err != nil {
		return Object{}, uuid.UUID{}, nil, 0, Error.New("unable to query object status: %w", err)
	}
	if sourceObject.StreamID != opts.StreamID {
		return Object{}, uuid.UUID{}, nil, 0, ErrObjectNotFound.New("object was changed during copy")
	}

	if len(ancestorStreamIDBytes) != 0 {
		// Source object already was a copy, the new copy becomes yet another copy of the existing ancestor
		ancestorStreamID, err = uuid.FromBytes(ancestorStreamIDBytes)
		if err != nil {
			return Object{}, uuid.UUID{}, nil, 0, err
		}
	} else {
		// Source object was not a copy, it will now become an ancestor (unless it has only inline segments)
		ancestorStreamID = sourceObject.StreamID
	}

	if rows.Next() {
		var _bogusBytes []byte
		destinationObject = &Object{}
		destinationObject.ProjectID = opts.ProjectID
		destinationObject.BucketName = opts.NewBucket
		destinationObject.ObjectKey = opts.NewEncryptedObjectKey
		// There is an object at the destination.
		// We will delete it before doing the copy
		err := rows.Scan(
			&destinationObject.StreamID,
			&destinationObject.ExpiresAt,
			&destinationObject.SegmentCount,
			&destinationObject.EncryptedMetadata,
			&destinationObject.TotalPlainSize, &destinationObject.TotalEncryptedSize, &destinationObject.FixedSegmentSize,
			encryptionParameters{&destinationObject.Encryption},
			&_bogusBytes,
			&destinationObject.Version,
			&highestVersion,
		)
		if err != nil {
			return Object{}, uuid.UUID{}, nil, 0, Error.New("error while reading existing object at destination: %w", err)
		}
	}

	if rows.Next() {
		return Object{}, uuid.UUID{}, nil, 0, Error.New("expected 1 or 2 rows, got 3 or more")
	}

	return sourceObject, ancestorStreamID, destinationObject, highestVersion + 1, nil
}
