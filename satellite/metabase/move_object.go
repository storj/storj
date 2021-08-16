package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/storj"
	"storj.io/private/tagsql"
)

// BeginMoveObjectResult holds data needed to finish move object.
type BeginMoveObjectResult struct {
	EncryptedKeysNonces []EncryptedKeyAndNonce
	StreamID            storj.StreamID
}

// EncryptedKeyAndNonce holds single segment encrypted key.
type EncryptedKeyAndNonce struct {
	EncryptedKeyNonce storj.Nonce
	EncryptedKey      []byte
}

// BeginMoveObject holds all needed for object move data.
type BeginMoveObject struct {
	ProjectID  []byte
	BucketName []byte
	ObjectKey  []byte
}

// BeginMoveObject get encryptedKeys to decrypts them with new ObjectKey.
func (db *DB) BeginMoveObject(ctx context.Context, opts BeginMoveObject) (result BeginMoveObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(db.db.QueryContext(ctx, `
		SELECT
			encrypted_key_nonce, encrypted_key, stream_id
		FROM segments
		WHERE
			project_id = $1 AND
	        bucket_name = $2 AND
            object_key = $3 AND
		ORDER BY position ASC
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var keys EncryptedKeyAndNonce

			err = rows.Scan(&keys.EncryptedKeyNonce, &keys.EncryptedKey, result.StreamID[:])
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			result.EncryptedKeysNonces = append(result.EncryptedKeysNonces, keys)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BeginMoveObjectResult{}, nil
		}

		return BeginMoveObjectResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	return result, nil
}

// FinishMoveObject ...
func (db *DB) FinishMoveObject(ctx context.Context, newObjectKey []byte, keysAndNonces []EncryptedKeyAndNonce) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}
