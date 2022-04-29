// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgtype"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// BeginCopyObjectResult holds data needed to finish copy object.
type BeginCopyObjectResult struct {
	StreamID                  uuid.UUID
	EncryptedMetadata         []byte
	EncryptedMetadataKeyNonce []byte
	EncryptedMetadataKey      []byte
	EncryptedKeysNonces       []EncryptedKeyAndNonce
	EncryptionParameters      storj.EncryptionParameters
}

// BeginCopyObject holds all data needed begin copy object method.
type BeginCopyObject struct {
	Version Version
	ObjectLocation
}

// BeginCopyObject collects all data needed to begin object copy procedure.
func (db *DB) BeginCopyObject(ctx context.Context, opts BeginCopyObject) (result BeginCopyObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectLocation.Verify(); err != nil {
		return BeginCopyObjectResult{}, err
	}

	if opts.Version <= 0 {
		return BeginCopyObjectResult{}, ErrInvalidRequest.New("Version invalid: %v", opts.Version)
	}

	var segmentCount int64

	err = db.db.QueryRowContext(ctx, `
		SELECT
			stream_id, encryption, segment_count,
			encrypted_metadata_encrypted_key, encrypted_metadata_nonce, encrypted_metadata
		FROM objects
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			status       = `+committedStatus,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version).
		Scan(
			&result.StreamID,
			encryptionParameters{&result.EncryptionParameters},
			&segmentCount,
			&result.EncryptedMetadataKey, &result.EncryptedMetadataKeyNonce, &result.EncryptedMetadata,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BeginCopyObjectResult{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return BeginCopyObjectResult{}, Error.New("unable to query object status: %w", err)
	}

	if segmentCount > CopySegmentLimit {
		return BeginCopyObjectResult{}, Error.New("object to copy has too many segments (%d). Limit is %d.", segmentCount, CopySegmentLimit)
	}

	err = withRows(db.db.QueryContext(ctx, `
		SELECT
			position, encrypted_key_nonce, encrypted_key
		FROM segments
		WHERE stream_id = $1
		ORDER BY stream_id, position ASC
	`, result.StreamID))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var keys EncryptedKeyAndNonce

			err = rows.Scan(&keys.Position, &keys.EncryptedKeyNonce, &keys.EncryptedKey)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			result.EncryptedKeysNonces = append(result.EncryptedKeysNonces, keys)
		}

		return nil
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return BeginCopyObjectResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	return result, nil
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
// TODO handle the case when the source and destination encrypted object keys are the same.
func (db *DB) FinishCopyObject(ctx context.Context, opts FinishCopyObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	originalObject := Object{}

	var ancestorStreamIDBytes []byte
	err = db.db.QueryRowContext(ctx, `
		SELECT
			objects.stream_id,
			expires_at,
			segment_count,
			encrypted_metadata,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			segment_copies.ancestor_stream_id
		FROM objects
		LEFT JOIN segment_copies ON objects.stream_id = segment_copies.stream_id
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			status       = `+committedStatus,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version).
		Scan(
			&originalObject.StreamID,
			&originalObject.ExpiresAt,
			&originalObject.SegmentCount,
			&originalObject.EncryptedMetadata,
			&originalObject.TotalPlainSize, &originalObject.TotalEncryptedSize, &originalObject.FixedSegmentSize,
			encryptionParameters{&originalObject.Encryption},
			&ancestorStreamIDBytes,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Object{}, storj.ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Object{}, Error.New("unable to query object status: %w", err)
	}
	originalObject.BucketName = opts.BucketName
	originalObject.ProjectID = opts.ProjectID
	originalObject.Version = opts.Version
	originalObject.Status = Committed

	if int(originalObject.SegmentCount) != len(opts.NewSegmentKeys) {
		return Object{}, ErrInvalidRequest.New("wrong amount of segments keys received (received %d, need %d)", originalObject.SegmentCount, len(opts.NewSegmentKeys))
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

	positions := make([]int64, originalObject.SegmentCount)

	rootPieceIDs := make([][]byte, originalObject.SegmentCount)

	expiresAts := make([]*time.Time, originalObject.SegmentCount)
	encryptedSizes := make([]int32, originalObject.SegmentCount)
	plainSizes := make([]int32, originalObject.SegmentCount)
	plainOffsets := make([]int64, originalObject.SegmentCount)
	inlineDatas := make([][]byte, originalObject.SegmentCount)

	redundancySchemes := make([]int64, originalObject.SegmentCount)
	// TODO: there are probably columns that we can skip
	// maybe it's possible to have the select and the insert in one query
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
			`, originalObject.StreamID, originalObject.SegmentCount))(func(rows tagsql.Rows) error {
		index := 0
		for rows.Next() {
			err = rows.Scan(
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

		if err = rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Object{}, Error.New("unable to copy object: %w", err)
	}

	onlyInlineSegments := true
	for index := range positions {
		if newSegments.Positions[index] != positions[index] {
			return Object{}, Error.New("missing new segment keys for segment %d", positions[index])
		}
		if onlyInlineSegments && (encryptedSizes[index] > 0) && len(inlineDatas[index]) == 0 {
			onlyInlineSegments = false
		}
	}

	copyMetadata := originalObject.EncryptedMetadata
	if opts.OverrideMetadata {
		copyMetadata = opts.NewEncryptedMetadata
	}

	copyObject := originalObject
	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		// TODO we need to handle metadata correctly (copy from original object or replace)
		row := db.db.QueryRowContext(ctx, `
			WITH existing_object AS (
				SELECT
					objects.stream_id,
					copies.stream_id AS new_ancestor,
					objects.segment_count
				FROM objects
				LEFT OUTER JOIN segment_copies copies ON objects.stream_id = copies.ancestor_stream_id
				WHERE
					project_id   = $1 AND
					bucket_name  = $2 AND
					object_key   = $3 AND
					version      = $4
			)
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
			) ON CONFLICT (project_id, bucket_name, object_key, version)
				DO UPDATE SET
					stream_id = $5,
					created_at = now(),
					expires_at = $6,
					status = `+committedStatus+`,
					segment_count = $7,
					encryption = $8,
					encrypted_metadata = $9,
					encrypted_metadata_nonce = $10,
					encrypted_metadata_encrypted_key = $11,
					total_plain_size = $12,
					total_encrypted_size = $13,
					fixed_segment_size = $14,
					zombie_deletion_deadline = NULL
			RETURNING
				created_at,
				(SELECT stream_id FROM existing_object LIMIT 1),
				(SELECT new_ancestor FROM existing_object LIMIT 1),
				(SELECT segment_count FROM existing_object LIMIT 1)`,
			opts.ProjectID, opts.NewBucket, opts.NewEncryptedObjectKey, opts.Version, opts.NewStreamID,
			originalObject.ExpiresAt, originalObject.SegmentCount,
			encryptionParameters{&originalObject.Encryption},
			copyMetadata, opts.NewEncryptedMetadataKeyNonce, opts.NewEncryptedMetadataKey,
			originalObject.TotalPlainSize, originalObject.TotalEncryptedSize, originalObject.FixedSegmentSize,
		)

		var existingObjStreamID *uuid.UUID
		var newAncestorStreamID *uuid.UUID
		var oldSegmentCount *int

		err = row.Scan(&copyObject.CreatedAt, &existingObjStreamID, &newAncestorStreamID, &oldSegmentCount)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		err = db.deleteExistingObjectSegments(ctx, tx, existingObjStreamID, newAncestorStreamID, oldSegmentCount)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		expiresAtElements := make([]pgtype.Timestamptz, len(expiresAts))
		for i, v := range expiresAts {
			if v == nil {
				expiresAtElements[i] = pgtype.Timestamptz{
					Status: pgtype.Null,
				}
			} else {
				expiresAtElements[i] = pgtype.Timestamptz{
					Time:   *v,
					Status: pgtype.Present,
				}
			}
		}

		expiresAtArray := &pgtype.TimestamptzArray{
			Elements:   expiresAtElements,
			Dimensions: []pgtype.ArrayDimension{{Length: int32(len(expiresAtElements)), LowerBound: 1}},
			Status:     pgtype.Present,
		}

		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				encrypted_key_nonce, encrypted_key,
				root_piece_id,
				redundancy,
				encrypted_size, plain_offset, plain_size,
				inline_data
			) SELECT
				$1, UNNEST($2::INT8[]), UNNEST($3::timestamptz[]),
				UNNEST($4::BYTEA[]), UNNEST($5::BYTEA[]),
				UNNEST($6::BYTEA[]),
				UNNEST($7::INT8[]),
				UNNEST($8::INT4[]), UNNEST($9::INT8[]),	UNNEST($10::INT4[]),
				UNNEST($11::BYTEA[])
		`, opts.NewStreamID, pgutil.Int8Array(newSegments.Positions), expiresAtArray,
			pgutil.ByteaArray(newSegments.EncryptedKeyNonces), pgutil.ByteaArray(newSegments.EncryptedKeys),
			pgutil.ByteaArray(rootPieceIDs),
			pgutil.Int8Array(redundancySchemes),
			pgutil.Int4Array(encryptedSizes), pgutil.Int8Array(plainOffsets), pgutil.Int4Array(plainSizes),
			pgutil.ByteaArray(inlineDatas),
		)
		if err != nil {
			return Error.New("unable to copy segments: %w", err)
		}

		if onlyInlineSegments {
			return nil
		}
		var ancestorStreamID uuid.UUID
		if len(ancestorStreamIDBytes) != 0 {
			ancestorStreamID, err = uuid.FromBytes(ancestorStreamIDBytes)
			if err != nil {
				return err
			}
		} else {
			ancestorStreamID = originalObject.StreamID
		}
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segment_copies (
				stream_id, ancestor_stream_id
			) VALUES (
				$1, $2
			)
		`, opts.NewStreamID, ancestorStreamID)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}
		return nil
	})

	if err != nil {
		return Object{}, err
	}

	copyObject.StreamID = opts.NewStreamID
	copyObject.BucketName = opts.NewBucket
	copyObject.ObjectKey = opts.NewEncryptedObjectKey
	copyObject.EncryptedMetadata = copyMetadata
	copyObject.EncryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
	if !opts.NewEncryptedMetadataKeyNonce.IsZero() {
		copyObject.EncryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce[:]
	}

	mon.Meter("finish_copy_object").Mark(1)

	return copyObject, nil
}

func (db *DB) deleteExistingObjectSegments(ctx context.Context, tx tagsql.Tx, existingObjStreamID *uuid.UUID, newAncestorStreamID *uuid.UUID, segmentCount *int) (err error) {
	if existingObjStreamID != nil && *segmentCount > 0 {
		if newAncestorStreamID == nil {
			_, err = db.db.ExecContext(ctx, `
			DELETE FROM segments WHERE stream_id = $1
		`, existingObjStreamID,
			)
			if err != nil {
				return Error.New("unable to copy segments: %w", err)
			}
			return nil
		}
		var infos deletedObjectInfo

		infos.SegmentCount = int32(*segmentCount)
		infos.PromotedAncestor = newAncestorStreamID
		infos.Segments = make([]deletedRemoteSegmentInfo, *segmentCount)

		var aliasPieces AliasPieces
		err = withRows(db.db.QueryContext(ctx, `
			DELETE FROM segments WHERE stream_id = $1
			RETURNING position, remote_alias_pieces, repaired_at
			`, existingObjStreamID))(func(rows tagsql.Rows) error {
			index := 0
			for rows.Next() {
				err = rows.Scan(
					&infos.Segments[index].Position,
					&aliasPieces,
					&infos.Segments[index].RepairedAt,
				)
				if err != nil {
					return err
				}
				infos.Segments[index].Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
				if err != nil {
					return Error.New("unable to copy object: %w", err)
				}
				index++
			}
			return rows.Err()
		})
		if err != nil {
			return Error.New("unable to copy segments: %w", err)
		}

		err = db.promoteNewAncestors(ctx, tx, []deletedObjectInfo{infos})
		if err != nil {
			return Error.New("unable to copy segments: %w", err)
		}
	}
	return nil
}
