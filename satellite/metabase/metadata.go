// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"

	"storj.io/common/uuid"
)

// EncryptedUserData contains user data that has been encrypted with the nonce and key.
type EncryptedUserData struct {
	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
	EncryptedETag                 []byte
}

// Verify checks whether the fields have been set correctly.
func (opts EncryptedUserData) Verify() error {
	if (opts.EncryptedMetadataNonce == nil) != (opts.EncryptedMetadataEncryptedKey == nil) {
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must always be set together")
	}

	hasEncryptedData := opts.EncryptedMetadata != nil || opts.EncryptedETag != nil
	hasEncryptionKey := opts.EncryptedMetadataNonce != nil && opts.EncryptedMetadataEncryptedKey != nil

	switch {
	case hasEncryptedData && !hasEncryptionKey:
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set when EncryptedMetadata or EncryptedETag are set")
	case !hasEncryptedData && hasEncryptionKey:
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be empty when EncryptedMetadata or EncryptedETag are empty")
	}

	return nil
}

// UpdateObjectLastCommittedMetadata contains arguments necessary for replacing an object metadata.
type UpdateObjectLastCommittedMetadata struct {
	ObjectLocation
	StreamID uuid.UUID

	EncryptedUserData
	// SetEncryptedETag is true for new uplink clients that know to send EncryptedETag.
	SetEncryptedETag bool
}

// Verify object stream fields.
func (obj *UpdateObjectLastCommittedMetadata) Verify() error {
	if err := obj.ObjectLocation.Verify(); err != nil {
		return err
	}
	if obj.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// UpdateObjectLastCommittedMetadata updates an object metadata.
func (db *DB) UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	affected, err := db.ChooseAdapter(opts.ProjectID).UpdateObjectLastCommittedMetadata(ctx, opts)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrObjectNotFound.New("object with specified version and committed status is missing")
	}

	if affected > 1 {
		db.log.Warn("object with multiple committed versions were found!",
			zap.Stringer("Project ID", opts.ProjectID), zap.Stringer("Bucket Name", opts.BucketName),
			zap.String("Object Key", string(opts.ObjectKey)), zap.Stringer("Stream ID", opts.StreamID))
		mon.Meter("multiple_committed_versions").Mark(1)
	}

	mon.Meter("object_update_metadata").Mark(int(affected))

	return nil
}

// UpdateObjectLastCommittedMetadata updates an object metadata.
func (p *PostgresAdapter) UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) (affected int64, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: So the issue is that during a multipart upload of an object,
	// uplink can update object metadata. If we add the arguments EncryptedMetadata
	// to CommitObject, they will need to account for them being optional.
	// Leading to scenarios where uplink calls update metadata, but wants to clear them
	// during commit object.

	if opts.SetEncryptedETag {
		result, err := p.db.ExecContext(ctx, `
			UPDATE objects SET
				encrypted_metadata_nonce         = $5,
				encrypted_metadata               = $6,
				encrypted_metadata_encrypted_key = $7,
				encrypted_etag                   = $8
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3) AND
				version IN (SELECT version FROM objects WHERE
					(project_id, bucket_name, object_key) = ($1, $2, $3) AND
					status <> `+statusPending+` AND
					(expires_at IS NULL OR expires_at > now())
					ORDER BY version desc
					LIMIT 1
				) AND
				stream_id    = $4 AND
				status       IN `+statusesCommitted,
			opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.StreamID,
			opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey, opts.EncryptedETag)
		if err != nil {
			return 0, Error.New("unable to update object metadata: %w", err)
		}

		affected, err = result.RowsAffected()
		if err != nil {
			return 0, Error.New("failed to get rows affected: %w", err)
		}
		return affected, nil
	} else {
		result, err := p.db.ExecContext(ctx, `
			UPDATE objects SET
				encrypted_metadata_nonce         = $5,
				encrypted_metadata               = $6,
				encrypted_metadata_encrypted_key = $7
			WHERE
				(project_id, bucket_name, object_key, stream_id) = ($1, $2, $3, $4) AND
				version IN (SELECT version FROM objects WHERE
					(project_id, bucket_name, object_key) = ($1, $2, $3) AND
					status <> `+statusPending+` AND
					(expires_at IS NULL OR expires_at > now())
					ORDER BY version desc
					LIMIT 1
				) AND
				status       IN `+statusesCommitted+` AND
				(encrypted_etag IS NULL OR length(encrypted_etag) = 0)
			`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.StreamID,
			opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey)
		if err != nil {
			return 0, Error.New("unable to update object metadata: %w", err)
		}

		affected, err = result.RowsAffected()
		if err != nil {
			return 0, Error.New("failed to get rows affected: %w", err)
		}
		return affected, nil
	}
}

// UpdateObjectLastCommittedMetadata updates an object metadata.
func (s *SpannerAdapter) UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) (affected int64, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: So the issue is that during a multipart upload of an object,
	// uplink can update object metadata. If we add the arguments EncryptedMetadata
	// to CommitObject, they will need to account for them being optional.
	// Leading to scenarios where uplink calls update metadata, but wants to clear them
	// during commit object.

	if opts.SetEncryptedETag {
		_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
			affected, err = tx.UpdateWithOptions(ctx, spanner.Statement{
				SQL: `
					UPDATE objects SET
						encrypted_metadata_nonce         = @encrypted_metadata_nonce,
						encrypted_metadata               = @encrypted_metadata,
						encrypted_metadata_encrypted_key = @encrypted_metadata_encrypted_key,
						encrypted_etag                   = @encrypted_etag
					WHERE
						(project_id, bucket_name, object_key, stream_id) = (@project_id, @bucket_name, @object_key, @stream_id) AND
						version IN (SELECT version FROM objects WHERE
							(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key) AND
							status <> ` + statusPending + ` AND
							(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
							ORDER BY version desc
							LIMIT 1
						) AND
						status IN ` + statusesCommitted + `
				`,
				Params: map[string]interface{}{
					"project_id":                       opts.ProjectID,
					"bucket_name":                      opts.BucketName,
					"object_key":                       []byte(opts.ObjectKey),
					"stream_id":                        opts.StreamID,
					"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
					"encrypted_metadata":               opts.EncryptedMetadata,
					"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
					"encrypted_etag":                   opts.EncryptedETag,
				},
			}, spanner.QueryOptions{RequestTag: "update-object-last-committed-metadata-with-encrypted-etag"})
			if err != nil {
				return Error.New("unable to update object metadata: %w", err)
			}
			return nil
		}, spanner.TransactionOptions{
			TransactionTag:              "update-object-last-committed-metadata-2",
			ExcludeTxnFromChangeStreams: true,
		})

		if err != nil {
			return 0, Error.Wrap(err)
		}
		return affected, nil
	} else {
		_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
			affected, err = tx.UpdateWithOptions(ctx, spanner.Statement{
				SQL: `
					UPDATE objects SET
						encrypted_metadata_nonce         = @encrypted_metadata_nonce,
						encrypted_metadata               = @encrypted_metadata,
						encrypted_metadata_encrypted_key = @encrypted_metadata_encrypted_key
					WHERE
						(project_id, bucket_name, object_key, stream_id) = (@project_id, @bucket_name, @object_key, @stream_id) AND
						version IN (SELECT version FROM objects WHERE
							(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key) AND
							status <> ` + statusPending + ` AND
							(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
							ORDER BY version desc
							LIMIT 1
						) AND
						status IN ` + statusesCommitted + ` AND
						(encrypted_etag IS NULL OR length(encrypted_etag) = 0)
				`,
				Params: map[string]interface{}{
					"project_id":                       opts.ProjectID,
					"bucket_name":                      opts.BucketName,
					"object_key":                       []byte(opts.ObjectKey),
					"stream_id":                        opts.StreamID,
					"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
					"encrypted_metadata":               opts.EncryptedMetadata,
					"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
				},
			}, spanner.QueryOptions{RequestTag: "update-object-last-committed-metadata"})
			if err != nil {
				return Error.New("unable to update object metadata: %w", err)
			}
			return nil
		}, spanner.TransactionOptions{
			TransactionTag:              "update-object-last-committed-metadata",
			ExcludeTxnFromChangeStreams: true,
		})

		if err != nil {
			return 0, Error.Wrap(err)
		}
		return affected, nil
	}

}
