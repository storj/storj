// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/dbutil/pgutil/pgerrcode"
)

var (
	// ErrInvalidRequest is used to indicate invalid requests.
	ErrInvalidRequest = errs.Class("metabase: invalid request")
	// ErrConflict is used to indicate conflict with the request.
	ErrConflict = errs.Class("metabase: conflict")
)

// BeginObjectNextVersion contains arguments necessary for starting an object upload.
type BeginObjectNextVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	Encryption storj.EncryptionParameters
}

// BeginObjectNextVersion adds a pending object to the database, with automatically assigned version.
func (db *DB) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (committed Version, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return -1, err
	}

	switch {
	case opts.Encryption.IsZero() || opts.Encryption.CipherSuite == storj.EncUnspecified:
		return -1, ErrInvalidRequest.New("Encryption is missing")
	case opts.Encryption.BlockSize <= 0:
		return -1, ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	case opts.Version != NextVersion:
		return -1, ErrInvalidRequest.New("Version should be metabase.NextVersion")
	}

	row := db.db.QueryRow(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline
		) VALUES (
			$1, $2, $3,
				coalesce((
					SELECT version + 1
					FROM objects
					WHERE project_id = $1 AND bucket_name = $2 AND object_key = $3
					ORDER BY version DESC
					LIMIT 1
				), 1), 
			$4, $5, $6,
			$7)
		RETURNING version
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline)

	var v int64
	if err := row.Scan(&v); err != nil {
		return -1, Error.New("unable to insert object: %w", err)
	}

	return Version(v), nil
}

// BeginObjectExactVersion contains arguments necessary for starting an object upload.
type BeginObjectExactVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	Encryption storj.EncryptionParameters
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (db *DB) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Version, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return -1, err
	}

	switch {
	case opts.Encryption.IsZero() || opts.Encryption.CipherSuite == storj.EncUnspecified:
		return -1, ErrInvalidRequest.New("Encryption is missing")
	case opts.Encryption.BlockSize <= 0:
		return -1, ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	case opts.Version == NextVersion:
		return -1, ErrInvalidRequest.New("Version should not be metabase.NextVersion")
	}

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline
		) values (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8
		)
	`,
		opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return -1, ErrConflict.New("object already exists")
		}
		return -1, Error.New("unable to insert object: %w", err)
	}

	return opts.Version, nil
}

// BeginSegment contains options to verify, whether a new segment upload can be started.
type BeginSegment struct {
	ObjectStream

	Position    SegmentPosition
	RootPieceID storj.PieceID
	Pieces      Pieces
}

// BeginSegment verifies, whether a new segment upload can be started.
func (db *DB) BeginSegment(ctx context.Context, opts BeginSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case opts.RootPieceID.IsZero():
		return ErrInvalidRequest.New("RootPieceID missing")
	case len(opts.Pieces) == 0:
		return ErrInvalidRequest.New("Pieces missing")
	}

	// TODO: verify opts.Pieces content.

	// NOTE: this isn't strictly necessary, since we can also fail this in CommitSegment.
	//       however, we should prevent creating segements for non-partial objects.

	// NOTE: these queries could be combined into one.

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return Error.New("failed BeginTx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errs.Combine(err, Error.Wrap(tx.Rollback()))
		}
	}()

	// Verify that object exists and is partial.
	var value int
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Error.New("pending object missing")
		}
		return Error.New("unable to query object status: %w", err)
	}

	// Verify that the segment does not exist.
	err = tx.QueryRow(ctx, `
		SELECT 1
		FROM segments WHERE
			stream_id = $1 AND
			position  = $2
	`, opts.StreamID, opts.Position).Scan(&value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Error.New("unable to query segments: %w", err)
	}
	err = nil // ignore any other err result (explicitly)

	err, committed = tx.Commit(), true
	if err != nil {
		return Error.New("unable to commit tx: %w", err)
	}

	return nil
}

// CommitSegment contains all necessary information about the segment.
type CommitSegment struct {
	ObjectStream

	Position    SegmentPosition
	RootPieceID storj.PieceID

	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	PlainOffset   int64 // offset in the original data stream
	PlainSize     int32 // size before encryption
	EncryptedSize int32 // segment size after encryption

	Redundancy storj.RedundancyScheme

	Pieces Pieces
}

// CommitSegment commits segment to the database.
func (db *DB) CommitSegment(ctx context.Context, opts CommitSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	switch {
	case opts.RootPieceID.IsZero():
		return ErrInvalidRequest.New("RootPieceID missing")
	case len(opts.Pieces) == 0:
		return ErrInvalidRequest.New("Pieces missing")
	case len(opts.EncryptedKey) == 0:
		return ErrInvalidRequest.New("EncryptedKey missing")
	case len(opts.EncryptedKeyNonce) == 0:
		return ErrInvalidRequest.New("EncryptedKeyNonce missing")
	case opts.EncryptedSize <= 0:
		return ErrInvalidRequest.New("EncryptedSize negative or zero")
	case opts.PlainSize <= 0:
		return ErrInvalidRequest.New("PlainSize negative or zero")
	case opts.PlainOffset < 0:
		return ErrInvalidRequest.New("PlainOffset negative")
	case opts.Redundancy.IsZero():
		return ErrInvalidRequest.New("Redundancy zero")
	}

	// TODO: verify opts.Pieces content is non-zero
	// TODO: verify opts.Pieces is compatible with opts.Redundancy

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return Error.New("failed BeginTx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errs.Combine(err, Error.Wrap(tx.Rollback()))
		}
	}()

	// Verify that object exists and is partial.
	var value int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Error.New("pending object missing")
		}
		return Error.New("unable to query object status: %w", err)
	}

	// Insert into segments.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO segments (
			stream_id, position,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			redundancy,
			remote_pieces
		) VALUES (
			$1, $2,
			$3, $4, $5,
			$6, $7, $8,
			$9,
			$10
		)`,
		opts.StreamID, opts.Position,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize,
		redundancyScheme{&opts.Redundancy},
		opts.Pieces,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return ErrConflict.New("segment already exists")
		}
		return Error.New("unable to insert segment: %w", err)
	}

	err, committed = tx.Commit(), true
	if err != nil {
		return Error.New("unable to commit tx: %w", err)
	}

	return nil
}

// CommitInlineSegment contains all necessary information about the segment.
type CommitInlineSegment struct {
	ObjectStream

	Position    SegmentPosition
	RootPieceID storj.PieceID // TODO: do we need this?

	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	PlainOffset   int64 // offset in the original data stream
	PlainSize     int32 // size before encryption
	EncryptedSize int32 // segment size after encryption

	Redundancy storj.RedundancyScheme // TODO: do we need this?

	InlineData []byte
}

// CommitInlineSegment commits inline segment to the database.
func (db *DB) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	// TODO: do we have a lower limit for inline data?

	switch {
	case opts.RootPieceID.IsZero():
		return ErrInvalidRequest.New("RootPieceID missing")
	case len(opts.InlineData) == 0:
		return ErrInvalidRequest.New("InlineData missing")
	case len(opts.EncryptedKey) == 0:
		return ErrInvalidRequest.New("EncryptedKey missing")
	case len(opts.EncryptedKeyNonce) == 0:
		return ErrInvalidRequest.New("EncryptedKeyNonce missing")
	case opts.EncryptedSize <= 0:
		return ErrInvalidRequest.New("EncryptedSize negative or zero")
	case opts.PlainSize <= 0:
		return ErrInvalidRequest.New("PlainSize negative or zero")
	case opts.PlainOffset < 0:
		return ErrInvalidRequest.New("PlainOffset negative")
	case opts.Redundancy.IsZero():
		return ErrInvalidRequest.New("Redundancy zero")
	}

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return Error.New("failed BeginTx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errs.Combine(err, Error.Wrap(tx.Rollback()))
		}
	}()

	// Verify that object exists and is partial.
	var value int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM objects WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Error.New("pending object missing")
		}
		return Error.New("unable to query object status: %w", err)
	}

	// Insert into segments.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO segments (
			stream_id, position,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			redundancy,
			inline_data
		) VALUES (
			$1, $2,
			$3, $4, $5,
			$6, $7, $8,
			$9,
			$10
		)`,
		opts.StreamID, opts.Position,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize,
		redundancyScheme{&opts.Redundancy},
		opts.InlineData,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return ErrConflict.New("segment already exists")
		}
		return Error.New("unable to insert segment: %w", err)
	}

	err, committed = tx.Commit(), true
	if err != nil {
		return Error.New("unable to commit tx: %w", err)
	}

	return nil
}

// CommitObject contains arguments necessary for committing an object.
type CommitObject struct {
	ObjectStream

	EncryptedMetadata      []byte
	EncryptedMetadataNonce []byte

	// TODO: proof
	Proofs []SegmentProof
}

// SegmentProof ensures that segments cannot be tampered with.
type SegmentProof struct{}

// CommitObject adds a pending object to the database.
func (db *DB) CommitObject(ctx context.Context, opts CommitObject) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	// TODO: deduplicate basic checks.
	switch {
	case len(opts.Proofs) > 0:
		return db.commitObjectWithProofs(ctx, opts)
	default:
		return db.commitObjectWithoutProofs(ctx, opts)
	}
}

func (db *DB) commitObjectWithoutProofs(ctx context.Context, opts CommitObject) (err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return Error.New("failed BeginTx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errs.Combine(err, Error.Wrap(tx.Rollback()))
		}
	}()

	// TODO: fetch info from segments

	result, err := tx.ExecContext(ctx, `
		UPDATE objects SET
			status = 1, -- committed
			segment_count = 0, -- TODO

			encrypted_metadata_nonce = $6,
			encrypted_metadata = $7,

			total_encrypted_size = 0, -- TODO
			fixed_segment_size = 0, -- TODO
			zombie_deletion_deadline = NULL
		WHERE
			project_id   = $1 AND
			bucket_name  = $2 AND
			object_key   = $3 AND
			version      = $4 AND
			stream_id    = $5 AND
			status       = 0;
	`, opts.ProjectID, opts.BucketName, []byte(opts.ObjectKey), opts.Version, opts.StreamID,
		opts.EncryptedMetadataNonce, opts.EncryptedMetadata)
	if err != nil {
		return Error.New("failed to update object: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Error.New("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return Error.New("object with specified version and pending status is missing")
	}

	// TODO: delete segments

	err = tx.Commit()
	committed = true

	return Error.Wrap(err)
}

func (db *DB) commitObjectWithProofs(ctx context.Context, opts CommitObject) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.New("unimplemented")
}
