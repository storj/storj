// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil/pgerrcode"
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
	NewEncryptedMetadataKeyNonce []byte
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
	case finishCopy.ObjectStream.StreamID == finishCopy.NewStreamID:
		return ErrInvalidRequest.New("StreamIDs are identical")
	case finishCopy.ObjectKey == finishCopy.NewEncryptedObjectKey:
		return ErrInvalidRequest.New("source and destination encrypted object key are identical")
	case len(finishCopy.NewEncryptedObjectKey) == 0:
		return ErrInvalidRequest.New("NewEncryptedObjectKey is missing")
	}

	if finishCopy.OverrideMetadata {
		if finishCopy.NewEncryptedMetadata == nil && (finishCopy.NewEncryptedMetadataKeyNonce != nil || finishCopy.NewEncryptedMetadataKey != nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
		} else if finishCopy.NewEncryptedMetadata != nil && (finishCopy.NewEncryptedMetadataKeyNonce == nil || finishCopy.NewEncryptedMetadataKey == nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
		}
	} else {
		switch {
		case len(finishCopy.NewEncryptedMetadataKeyNonce) == 0:
			return ErrInvalidRequest.New("EncryptedMetadataKeyNonce is missing")
		case len(finishCopy.NewEncryptedMetadataKey) == 0:
			return ErrInvalidRequest.New("EncryptedMetadataKey is missing")
		}
	}

	return nil
}

// FinishCopyObject accepts new encryption keys for copied object and insert the corresponding new object ObjectKey and segments EncryptedKey.
// TODO should be in one transaction.
// TODO handle the case when the source and destination encrypted object keys are the same.
func (db *DB) FinishCopyObject(ctx context.Context, opts FinishCopyObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	originalObject, err := db.GetObjectExactVersion(ctx, GetObjectExactVersion{
		opts.Version,
		opts.Location(),
	})
	if err != nil {
		return Object{}, errs.Wrap(err)
	}

	if int(originalObject.SegmentCount) != len(opts.NewSegmentKeys) {
		return Object{}, ErrInvalidRequest.New("wrong amount of segments keys received (received %d, need %d)", originalObject.SegmentCount, len(opts.NewSegmentKeys))
	}

	var newSegmentKeys struct {
		Positions          []int64
		EncryptedKeys      [][]byte
		EncryptedKeyNonces [][]byte
	}

	for _, u := range opts.NewSegmentKeys {
		newSegmentKeys.EncryptedKeys = append(newSegmentKeys.EncryptedKeys, u.EncryptedKey)
		newSegmentKeys.EncryptedKeyNonces = append(newSegmentKeys.EncryptedKeyNonces, u.EncryptedKeyNonce)
		newSegmentKeys.Positions = append(newSegmentKeys.Positions, int64(u.Position.Encode()))
	}

	segments := make([]Segment, originalObject.SegmentCount)
	positions := make([]int64, originalObject.SegmentCount)

	// TODO: there are probably columns that we can skip
	// maybe it's possible to have the select and the insert in one query
	err = withRows(db.db.QueryContext(ctx, `
			SELECT
				position,
				expires_at, repaired_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size,
				encrypted_etag,
				redundancy,
				inline_data,
				placement
			FROM segments
			WHERE stream_id = $1
			ORDER BY position ASC
			LIMIT  $2
			`, originalObject.StreamID, originalObject.SegmentCount))(func(rows tagsql.Rows) error {
		index := 0
		for rows.Next() {
			err = rows.Scan(
				&segments[index].Position,
				&segments[index].ExpiresAt, &segments[index].RepairedAt,
				&segments[index].RootPieceID, &segments[index].EncryptedKeyNonce, &segments[index].EncryptedKey,
				&segments[index].EncryptedSize, &segments[index].PlainOffset, &segments[index].PlainSize,
				&segments[index].EncryptedETag,
				redundancyScheme{&segments[index].Redundancy},
				&segments[index].InlineData,
				&segments[index].Placement,
			)
			if err != nil {
				return err
			}
			positions[index] = int64(segments[index].Position.Encode())
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

	for index := range segments {
		if newSegmentKeys.Positions[index] != int64(segments[index].Position.Encode()) {
			return Object{}, Error.New("missing new segment keys for segment %d", int64(segments[index].Position.Encode()))
		}
	}

	copyMetadata := originalObject.EncryptedMetadata
	if opts.OverrideMetadata {
		copyMetadata = opts.NewEncryptedMetadata
	}

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		// TODO we need to handle metadata correctly (copy from original object or replace)
		_, err = db.db.ExecContext(ctx, `
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
			)`,
			opts.ProjectID, opts.NewBucket, opts.NewEncryptedObjectKey, opts.Version, opts.NewStreamID,
			originalObject.ExpiresAt, originalObject.SegmentCount,
			encryptionParameters{&originalObject.Encryption},
			copyMetadata, opts.NewEncryptedMetadataKeyNonce, opts.NewEncryptedMetadataKey,
			originalObject.TotalPlainSize, originalObject.TotalEncryptedSize, originalObject.FixedSegmentSize,
		)
		if err != nil {
			if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
				return ErrObjectAlreadyExists.New("")
			}
			return Error.New("unable to copy object: %w", err)
		}

		// TODO: optimize - we should do a bulk insert
		for index, originalSegment := range segments {
			_, err = db.db.ExecContext(ctx, `
				INSERT INTO segments (
					stream_id, position,
					encrypted_key_nonce, encrypted_key,
					root_piece_id, -- non-null constraint
					redundancy,
					encrypted_size, plain_offset, plain_size,
					inline_data
				) VALUES (
					$1, $2,
					$3, $4,
					$5,
					$6,
					$7, $8,	$9,
					$10
				)
			`, opts.NewStreamID, originalSegment.Position.Encode(),
				newSegmentKeys.EncryptedKeyNonces[index], newSegmentKeys.EncryptedKeys[index],
				originalSegment.RootPieceID,
				redundancyScheme{&originalSegment.Redundancy},
				originalSegment.EncryptedSize, originalSegment.PlainOffset, originalSegment.PlainSize,
				originalSegment.InlineData,
			)
			if err != nil {
				return Error.New("unable to copy segment: %w", err)
			}
		}

		// TODO : we need flatten references
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segment_copies (
				stream_id, ancestor_stream_id
			) VALUES (
				$1, $2
			)
		`, opts.NewStreamID, originalObject.StreamID)
		if err != nil {
			return Error.New("unable to copy object: %w", err)
		}

		return nil
	})
	if err != nil {
		return Object{}, err
	}

	copyObject := originalObject
	copyObject.StreamID = opts.NewStreamID
	copyObject.BucketName = opts.NewBucket
	copyObject.ObjectKey = opts.NewEncryptedObjectKey
	copyObject.EncryptedMetadata = copyMetadata
	copyObject.EncryptedMetadataEncryptedKey = opts.NewEncryptedMetadataKey
	copyObject.EncryptedMetadataNonce = opts.NewEncryptedMetadataKeyNonce

	mon.Meter("finish_copy_object").Mark(1)

	return copyObject, nil
}
