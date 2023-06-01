// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/uuid"
)

// UpdateObjectMetadata contains arguments necessary for replacing an object metadata.
type UpdateObjectMetadata struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	StreamID   uuid.UUID

	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
}

// Verify object stream fields.
func (obj *UpdateObjectMetadata) Verify() error {
	switch {
	case obj.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case obj.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case len(obj.ObjectKey) == 0:
		return ErrInvalidRequest.New("ObjectKey missing")
	case obj.StreamID.IsZero():
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// UpdateObjectMetadata updates an object metadata.
func (db *DB) UpdateObjectMetadata(ctx context.Context, opts UpdateObjectMetadata) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	// TODO So the issue is that during a multipart upload of an object,
	// uplink can update object metadata. If we add the arguments EncryptedMetadata
	// to CommitObject, they will need to account for them being optional.
	// Leading to scenarios where uplink calls update metadata, but wants to clear them
	// during commit object.
	result, err := db.db.ExecContext(ctx, `
		UPDATE objects SET
			encrypted_metadata_nonce         = $5,
			encrypted_metadata               = $6,
			encrypted_metadata_encrypted_key = $7
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version IN (SELECT version FROM objects WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				status       = `+committedStatus+` AND
				(expires_at IS NULL OR expires_at > now())
				ORDER BY version desc
			) AND
			stream_id    = $4 AND
			status       = `+committedStatus,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
		opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey)
	if err != nil {
		return Error.New("unable to update object metadata: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Error.New("failed to get rows affected: %w", err)
	}

	if affected == 0 {
		return ErrObjectNotFound.New("object with specified version and committed status is missing")
	}

	if affected > 1 {
		db.log.Warn("object with multiple committed versions were found!",
			zap.Stringer("Project ID", opts.ProjectID), zap.String("Bucket Name", opts.BucketName),
			zap.String("Object Key", string(opts.ObjectKey)), zap.Stringer("Stream ID", opts.StreamID))
		mon.Meter("multiple_committed_versions").Mark(1)
	}

	mon.Meter("object_update_metadata").Mark(int(affected))

	return nil
}
