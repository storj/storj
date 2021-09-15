// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// BeginMoveObjectResult holds data needed to finish move object.
type BeginMoveObjectResult struct {
	StreamID uuid.UUID
	// TODO we need metadata becase of an uplink issue with how we are storing key and nonce
	EncryptedMetadata         []byte
	EncryptedMetadataKeyNonce []byte
	EncryptedMetadataKey      []byte
	EncryptedKeysNonces       []EncryptedKeyAndNonce
	EncryptionParameters      storj.EncryptionParameters
}

// EncryptedKeyAndNonce holds single segment position, encrypted key and nonce.
type EncryptedKeyAndNonce struct {
	Position          SegmentPosition
	EncryptedKeyNonce []byte
	EncryptedKey      []byte
}

// BeginMoveObject holds all data needed begin move object method.
type BeginMoveObject struct {
	Version Version
	ObjectLocation
}

// BeginMoveObject collects all data needed to begin object move procedure.
func (db *DB) BeginMoveObject(ctx context.Context, opts BeginMoveObject) (result BeginMoveObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectLocation.Verify(); err != nil {
		return BeginMoveObjectResult{}, err
	}

	if opts.Version <= 0 {
		return BeginMoveObjectResult{}, ErrInvalidRequest.New("Version invalid: %v", opts.Version)
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
		opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version).
		Scan(
			&result.StreamID,
			encryptionParameters{&result.EncryptionParameters},
			&segmentCount,
			&result.EncryptedMetadataKey, &result.EncryptedMetadataKeyNonce, &result.EncryptedMetadata,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BeginMoveObjectResult{}, storj.ErrObjectNotFound.Wrap(err)
		}
		return BeginMoveObjectResult{}, Error.New("unable to query object status: %w", err)
	}

	if segmentCount > MoveLimit {
		return BeginMoveObjectResult{}, Error.New("segment count of chosen object is beyond limit")
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
		return BeginMoveObjectResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	return result, nil
}
