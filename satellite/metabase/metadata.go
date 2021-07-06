// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/storj"
)

// SetObjectMetadataLatestVersion contains arguments necessary for replacing an object metadata.
type SetObjectMetadataLatestVersion struct {
	ObjectLocation

	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
}

// SetObjectMetadataLatestVersion replaces an object metadata.
func (db *DB) SetObjectMetadataLatestVersion(ctx context.Context, opts SetObjectMetadataLatestVersion) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectLocation.Verify(); err != nil {
		return err
	}

	// TODO So the issue is that during a multipart upload of an object,
	// uplink can update object metadata. If we add the arguments EncryptedMetadata
	// to CommitObject, they will need to account for them being optional.
	// Leading to scenarios where uplink calls update metadata, but wants to clear them
	// during commit object.
	result, err := db.db.ExecContext(ctx, `
		UPDATE objects SET
			encrypted_metadata_nonce         = $4,
			encrypted_metadata               = $5,
			encrypted_metadata_encrypted_key = $6
		FROM (
			SELECT version, stream_id FROM objects WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				status       = `+committedStatus+`
			ORDER BY version DESC
			LIMIT 1
		) AS latest_object
		WHERE
			project_id        = $1 AND
			bucket_name       = $2 AND
			object_key        = $3 AND
			objects.version   = latest_object.version AND
			objects.stream_id = latest_object.stream_id AND
			status            = `+committedStatus,
		opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey),
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
			Error.New("object with specified committed status is missing"),
		)
	}

	mon.Meter("object_update_metadata").Mark(1)

	return nil
}
