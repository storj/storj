// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/storj"
)

// UpdateObjectMetadata contains arguments necessary for replacing an object metadata.
type UpdateObjectMetadata struct {
	ObjectStream

	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
}

// UpdateObjectMetadata updates an object metadata.
func (db *DB) UpdateObjectMetadata(ctx context.Context, opts UpdateObjectMetadata) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.ObjectStream.Version <= 0 {
		return ErrInvalidRequest.New("Version invalid: %v", opts.Version)
	}

	// TODO So the issue is that during a multipart upload of an object,
	// uplink can update object metadata. If we add the arguments EncryptedMetadata
	// to CommitObject, they will need to account for them being optional.
	// Leading to scenarios where uplink calls update metadata, but wants to clear them
	// during commit object.
	result, err := db.db.ExecContext(ctx, `
		UPDATE objects SET
			encrypted_metadata_nonce         = $6,
			encrypted_metadata               = $7,
			encrypted_metadata_encrypted_key = $8
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = `+committedStatus,
		opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID,
		opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey)
	if err != nil {
		return Error.New("unable to update object metadata: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Error.New("failed to get rows affected: %w", err)
	}

	if affected == 0 {
		return storj.ErrObjectNotFound.Wrap(
			Error.New("object with specified version and committed status is missing"),
		)
	}

	mon.Meter("object_update_metadata").Mark(1)

	return nil
}
