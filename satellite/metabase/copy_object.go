// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

	// NewDisallowDelete indicates whether the user is allowed to delete an existing unversioned object.
	NewDisallowDelete bool

	// NewVersioned indicates that the object allows multiple versions.
	NewVersioned bool

	// VerifyLimits holds a callback by which the caller can interrupt the copy
	// if it turns out completing the copy would exceed a limit.
	// It will be called only once.
	VerifyLimits func(encryptedObjectSize int64, nSegments int64) error
}

// NewLocation returns the new object location.
func (finishCopy FinishCopyObject) NewLocation() ObjectLocation {
	return ObjectLocation{
		ProjectID:  finishCopy.ProjectID,
		BucketName: finishCopy.NewBucket,
		ObjectKey:  finishCopy.NewEncryptedObjectKey,
	}
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

	var precommit precommitConstraintResult
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		sourceObject, err := getObjectNonPendingExactVersion(ctx, tx, opts)
		if err != nil {
			if ErrObjectNotFound.Has(err) {
				return ErrObjectNotFound.New("source object not found")
			}
			return err
		}
		if sourceObject.StreamID != opts.StreamID {
			// TODO(versioning): should we report it as "not found" instead?
			return ErrObjectNotFound.New("object was changed during copy")
		}
		if sourceObject.Status.IsDeleteMarker() {
			return ErrMethodNotAllowed.New("copying delete marker is not allowed")
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

		precommit, err = db.precommitConstraint(ctx, precommitConstraint{
			Location:       opts.NewLocation(),
			Versioned:      opts.NewVersioned,
			DisallowDelete: opts.NewDisallowDelete,
		}, tx)
		if err != nil {
			return Error.Wrap(err)
		}

		newStatus := committedWhereVersioned(opts.NewVersioned)

		// TODO we need to handle metadata correctly (copy from original object or replace)
		row := tx.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status, expires_at, segment_count,
				encryption,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				zombie_deletion_deadline
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8,
				$9,
				$10, $11, $12,
				$13, $14, $15, null
			)
			RETURNING
				created_at`,
			opts.ProjectID, []byte(opts.NewBucket), opts.NewEncryptedObjectKey, precommit.HighestVersion+1, opts.NewStreamID,
			newStatus, sourceObject.ExpiresAt, sourceObject.SegmentCount,
			encryptionParameters{&sourceObject.Encryption},
			copyMetadata, opts.NewEncryptedMetadataKeyNonce, opts.NewEncryptedMetadataKey,
			sourceObject.TotalPlainSize, sourceObject.TotalEncryptedSize, sourceObject.FixedSegmentSize,
		)

		newObject = sourceObject
		newObject.Version = precommit.HighestVersion + 1
		newObject.Status = newStatus

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

	precommit.submitMetrics()
	mon.Meter("finish_copy_object").Mark(1)

	return newObject, nil
}

// getObjectNonPendingExactVersion returns object information for exact version.
//
// Note: this returns both committed objects and delete markers.
func getObjectNonPendingExactVersion(ctx context.Context, tx tagsql.Tx, opts FinishCopyObject) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	object := Object{}
	err = tx.QueryRowContext(ctx, `
		SELECT
			stream_id, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4) AND
			status <> `+statusPending+` AND
			(expires_at IS NULL OR expires_at > now())`,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version).
		Scan(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			&object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Object{}, Error.New("unable to query object status: %w", err)
	}

	object.ProjectID = opts.ProjectID
	object.BucketName = opts.BucketName
	object.ObjectKey = opts.ObjectKey
	object.Version = opts.Version

	return object, nil
}
