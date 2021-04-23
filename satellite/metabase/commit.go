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
	"storj.io/private/dbutil/pgutil/pgerrcode"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// we need to disable PlainSize validation for old uplinks.
const validatePlainSize = false

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

	if opts.Version != NextVersion {
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
	`, opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.StreamID,
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
func (db *DB) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return Object{}, err
	}

	if opts.Version == NextVersion {
		return Object{}, ErrInvalidRequest.New("Version should not be metabase.NextVersion")
	}

	object := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			Version:    opts.Version,
			StreamID:   opts.StreamID,
		},
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
	}

	err = db.db.QueryRow(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline
		) values (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8
		)
		RETURNING status, created_at
	`, opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline).
		Scan(
			&object.Status, &object.CreatedAt,
		)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return Object{}, ErrConflict.New("object already exists")
		}
		return Object{}, Error.New("unable to insert object: %w", err)
	}

	return object, nil
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

	if err := opts.Pieces.Verify(); err != nil {
		return err
	}

	if opts.RootPieceID.IsZero() {
		return ErrInvalidRequest.New("RootPieceID missing")
	}

	// NOTE: this isn't strictly necessary, since we can also fail this in CommitSegment.
	//       however, we should prevent creating segements for non-partial objects.

	// NOTE: these queries could be combined into one.

	return txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
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
				status       = `+pendingStatus,
			opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID).Scan(&value)
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
		err = nil //nolint: ineffassign, ignore any other err result (explicitly)

		return nil
	})
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

	EncryptedETag []byte

	Redundancy storj.RedundancyScheme

	Pieces Pieces
}

// CommitSegment commits segment to the database.
func (db *DB) CommitSegment(ctx context.Context, opts CommitSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if err := opts.Pieces.Verify(); err != nil {
		return err
	}

	switch {
	case opts.RootPieceID.IsZero():
		return ErrInvalidRequest.New("RootPieceID missing")
	case len(opts.EncryptedKey) == 0:
		return ErrInvalidRequest.New("EncryptedKey missing")
	case len(opts.EncryptedKeyNonce) == 0:
		return ErrInvalidRequest.New("EncryptedKeyNonce missing")
	case opts.EncryptedSize <= 0:
		return ErrInvalidRequest.New("EncryptedSize negative or zero")
	case opts.PlainSize <= 0 && validatePlainSize:
		return ErrInvalidRequest.New("PlainSize negative or zero")
	case opts.PlainOffset < 0:
		return ErrInvalidRequest.New("PlainOffset negative")
	case opts.Redundancy.IsZero():
		return ErrInvalidRequest.New("Redundancy zero")
	}

	if len(opts.Pieces) < int(opts.Redundancy.OptimalShares) {
		return ErrInvalidRequest.New("number of pieces is less than redundancy optimal shares value")
	}

	aliasPieces, err := db.aliasCache.ConvertPiecesToAliases(ctx, opts.Pieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	// Verify that object exists and is partial.
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO segments (
			stream_id, position,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size, encrypted_etag,
			redundancy,
			remote_alias_pieces
		) VALUES (
			(SELECT stream_id
				FROM objects WHERE
					project_id   = $11 AND
					bucket_name  = $12 AND
					object_key   = $13 AND
					version      = $14 AND
					stream_id    = $15 AND
					status       = `+pendingStatus+
		`	), $1,
			$2, $3, $4,
			$5, $6, $7, $8,
			$9,
			$10
		)`, opts.Position,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		redundancyScheme{&opts.Redundancy},
		aliasPieces,
		opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return Error.New("pending object missing")
		}
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return ErrConflict.New("segment already exists")
		}
		return Error.New("unable to insert segment: %w", err)
	}
	return nil
}

// CommitInlineSegment contains all necessary information about the segment.
type CommitInlineSegment struct {
	ObjectStream

	Position SegmentPosition

	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	PlainOffset   int64 // offset in the original data stream
	PlainSize     int32 // size before encryption
	EncryptedETag []byte

	InlineData []byte
}

// CommitInlineSegment commits inline segment to the database.
func (db *DB) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	// TODO: do we have a lower limit for inline data?
	// TODO should we move check for max inline segment from metainfo here

	switch {
	case len(opts.EncryptedKey) == 0:
		return ErrInvalidRequest.New("EncryptedKey missing")
	case len(opts.EncryptedKeyNonce) == 0:
		return ErrInvalidRequest.New("EncryptedKeyNonce missing")
	case opts.PlainSize <= 0 && validatePlainSize:
		return ErrInvalidRequest.New("PlainSize negative or zero")
	case opts.PlainOffset < 0:
		return ErrInvalidRequest.New("PlainOffset negative")
	}

	// Verify that object exists and is partial.
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO segments (
			stream_id, position,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size, encrypted_etag,
			inline_data
		) VALUES (
			(SELECT stream_id
				FROM objects WHERE
					project_id   = $10 AND
					bucket_name  = $11 AND
					object_key   = $12 AND
					version      = $13 AND
					stream_id    = $14 AND
					status       = `+pendingStatus+
		`	), $1,
			$2, $3, $4,
			$5, $6, $7, $8,
			$9
		)`, opts.Position,
		storj.PieceID{}, opts.EncryptedKeyNonce, opts.EncryptedKey,
		len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		opts.InlineData,
		opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return Error.New("pending object missing")
		}
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return ErrConflict.New("segment already exists")
		}
		return Error.New("unable to insert segment: %w", err)
	}
	return nil
}

// CommitObject contains arguments necessary for committing an object.
type CommitObject struct {
	ObjectStream

	Encryption storj.EncryptionParameters

	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
}

// CommitObject adds a pending object to the database.
func (db *DB) CommitObject(ctx context.Context, opts CommitObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return Object{}, err
	}

	if opts.Encryption.CipherSuite != storj.EncUnspecified && opts.Encryption.BlockSize <= 0 {
		return Object{}, ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	}

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		segments, err := fetchSegmentsForCommit(ctx, tx, opts.StreamID)
		if err != nil {
			return Error.New("failed to fetch segments: %w", err)
		}

		finalSegments := convertToFinalSegments(segments)
		err = updateSegmentOffsets(ctx, tx, opts.StreamID, finalSegments)
		if err != nil {
			return Error.New("failed to update segments: %w", err)
		}

		// TODO: would we even need this when we make main index plain_offset?
		fixedSegmentSize := int32(0)
		if len(finalSegments) > 0 {
			fixedSegmentSize = finalSegments[0].PlainSize
			for i, seg := range finalSegments {
				if seg.Position.Part != 0 || seg.Position.Index != uint32(i) {
					fixedSegmentSize = -1
					break
				}
				if i < len(finalSegments)-1 && seg.PlainSize != fixedSegmentSize {
					fixedSegmentSize = -1
					break
				}
			}
		}

		var totalPlainSize, totalEncryptedSize int64
		for _, seg := range finalSegments {
			totalPlainSize += int64(seg.PlainSize)
			totalEncryptedSize += int64(seg.EncryptedSize)
		}

		err = tx.QueryRow(ctx, `
			UPDATE objects SET
				status =`+committedStatus+`,
				segment_count = $6,

				encrypted_metadata_nonce         = $7,
				encrypted_metadata               = $8,
				encrypted_metadata_encrypted_key = $9,

				total_plain_size     = $10,
				total_encrypted_size = $11,
				fixed_segment_size   = $12,
				zombie_deletion_deadline = NULL,

				-- TODO should we allow to override existing encryption parameters or return error if don't match with opts?
				encryption = CASE
					WHEN objects.encryption = 0 AND $13 <> 0 THEN $13
					WHEN objects.encryption = 0 AND $13 = 0 THEN NULL
					ELSE objects.encryption
				END
			WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				version      = $4 AND
				stream_id    = $5 AND
				status       = `+pendingStatus+`
			RETURNING
				created_at, expires_at,
				encryption;
		`, opts.ProjectID, []byte(opts.BucketName), []byte(opts.ObjectKey), opts.Version, opts.StreamID,
			len(segments),
			opts.EncryptedMetadataNonce, opts.EncryptedMetadata, opts.EncryptedMetadataEncryptedKey,
			totalPlainSize,
			totalEncryptedSize,
			fixedSegmentSize,
			encryptionParameters{&opts.Encryption},
		).
			Scan(
				&object.CreatedAt, &object.ExpiresAt,
				encryptionParameters{&object.Encryption},
			)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return storj.ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
			} else if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
				// TODO maybe we should check message if 'encryption' label is there
				return ErrInvalidRequest.New("Encryption is missing")
			}
			return Error.New("failed to update object: %w", err)
		}

		object.StreamID = opts.StreamID
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		object.Version = opts.Version
		object.Status = Committed
		object.SegmentCount = int32(len(segments))
		object.EncryptedMetadataNonce = opts.EncryptedMetadataNonce
		object.EncryptedMetadata = opts.EncryptedMetadata
		object.EncryptedMetadataEncryptedKey = opts.EncryptedMetadataEncryptedKey
		object.TotalPlainSize = totalPlainSize
		object.TotalEncryptedSize = totalEncryptedSize
		object.FixedSegmentSize = fixedSegmentSize
		return nil
	})
	if err != nil {
		return Object{}, err
	}
	return object, nil
}

// UpdateObjectMetadata contains arguments necessary for updating an object metadata.
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

	return nil
}
