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
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/private/dbutil/pgutil/pgerrcode"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// we need to disable PlainSize validation for old uplinks.
const validatePlainSize = false

const defaultZombieDeletionPeriod = 24 * time.Hour

var (
	// ErrObjectNotFound is used to indicate that the object does not exist.
	ErrObjectNotFound = errs.Class("object not found")
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

	EncryptedMetadata             []byte // optional
	EncryptedMetadataNonce        []byte // optional
	EncryptedMetadataEncryptedKey []byte // optional

	Encryption storj.EncryptionParameters

	// UsePendingObjectsTable was added to options not metabase configuration
	// to be able to test scenarios with pending object in pending_objects and
	// objects table with the same test case.
	UsePendingObjectsTable bool
}

// Verify verifies get object request fields.
func (opts *BeginObjectNextVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version != NextVersion {
		return ErrInvalidRequest.New("Version should be metabase.NextVersion")
	}

	if opts.EncryptedMetadata == nil && (opts.EncryptedMetadataNonce != nil || opts.EncryptedMetadataEncryptedKey != nil) {
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
	} else if opts.EncryptedMetadata != nil && (opts.EncryptedMetadataNonce == nil || opts.EncryptedMetadataEncryptedKey == nil) {
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
	}
	return nil
}

// BeginObjectNextVersion adds a pending object to the database, with automatically assigned version.
// TODO at the end of transition to pending_objects table we can rename this metod to just BeginObject.
func (db *DB) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	if opts.ZombieDeletionDeadline == nil {
		deadline := time.Now().Add(defaultZombieDeletionPeriod)
		opts.ZombieDeletionDeadline = &deadline
	}

	object = Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			StreamID:   opts.StreamID,
		},
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
	}

	if opts.UsePendingObjectsTable {
		object.Status = Pending
		object.Version = DefaultVersion

		if err := db.db.QueryRowContext(ctx, `
			INSERT INTO pending_objects (
				project_id, bucket_name, object_key, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
			) VALUES (
				$1, $2, $3, $4,
				$5, $6,
				$7,
				$8, $9, $10)
			RETURNING created_at
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
			opts.ExpiresAt, encryptionParameters{&opts.Encryption},
			opts.ZombieDeletionDeadline,
			opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey,
		).Scan(&object.CreatedAt); err != nil {
			if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
				return Object{}, Error.Wrap(ErrObjectAlreadyExists.New(""))
			}
			return Object{}, Error.New("unable to insert object: %w", err)
		}
	} else {
		if err := db.db.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
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
				$7,
				$8, $9, $10)
			RETURNING status, version, created_at
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
			opts.ExpiresAt, encryptionParameters{&opts.Encryption},
			opts.ZombieDeletionDeadline,
			opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey,
		).Scan(&object.Status, &object.Version, &object.CreatedAt); err != nil {
			return Object{}, Error.New("unable to insert object: %w", err)
		}
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginObjectExactVersion contains arguments necessary for starting an object upload.
type BeginObjectExactVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedMetadata             []byte // optional
	EncryptedMetadataNonce        []byte // optional
	EncryptedMetadataEncryptedKey []byte // optional

	Encryption storj.EncryptionParameters
}

// Verify verifies get object reqest fields.
func (opts *BeginObjectExactVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version == NextVersion {
		return ErrInvalidRequest.New("Version should not be metabase.NextVersion")
	}

	if opts.EncryptedMetadata == nil && (opts.EncryptedMetadataNonce != nil || opts.EncryptedMetadataEncryptedKey != nil) {
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
	} else if opts.EncryptedMetadata != nil && (opts.EncryptedMetadataNonce == nil || opts.EncryptedMetadataEncryptedKey == nil) {
		return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
	}
	return nil
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (db *DB) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	if opts.ZombieDeletionDeadline == nil {
		deadline := time.Now().Add(defaultZombieDeletionPeriod)
		opts.ZombieDeletionDeadline = &deadline
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

	err = db.db.QueryRowContext(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline,
			encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8,
			$9, $10, $11
		)
		RETURNING status, created_at
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey,
	).Scan(
		&object.Status, &object.CreatedAt,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return Object{}, Error.Wrap(ErrObjectAlreadyExists.New(""))
		}
		return Object{}, Error.New("unable to insert object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginSegment contains options to verify, whether a new segment upload can be started.
type BeginSegment struct {
	ObjectStream

	Position SegmentPosition

	// TODO: unused field, can remove
	RootPieceID storj.PieceID

	Pieces Pieces

	UsePendingObjectsTable bool
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

	// Verify that object exists and is partial.
	var value int
	if opts.UsePendingObjectsTable {
		err = db.db.QueryRowContext(ctx, `
			SELECT 1
			FROM pending_objects WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				stream_id    = $4
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID).Scan(&value)
	} else {
		err = db.db.QueryRowContext(ctx, `
			SELECT 1
			FROM objects WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				version      = $4 AND
				stream_id    = $5 AND
				status       = `+pendingStatus,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID).Scan(&value)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPendingObjectMissing.New("")
		}
		return Error.New("unable to query object status: %w", err)
	}

	mon.Meter("segment_begin").Mark(1)

	return nil
}

// CommitSegment contains all necessary information about the segment.
type CommitSegment struct {
	ObjectStream

	Position    SegmentPosition
	RootPieceID storj.PieceID

	ExpiresAt *time.Time

	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	PlainOffset   int64 // offset in the original data stream
	PlainSize     int32 // size before encryption
	EncryptedSize int32 // segment size after encryption

	EncryptedETag []byte

	Redundancy storj.RedundancyScheme

	Pieces Pieces

	Placement storj.PlacementConstraint

	UsePendingObjectsTable bool
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

	aliasPieces, err := db.aliasCache.EnsurePiecesToAliases(ctx, opts.Pieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	// second part will be removed when there will be no pending_objects in objects table.
	// Verify that object exists and is partial.
	if opts.UsePendingObjectsTable {
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				redundancy,
				remote_alias_pieces,
				placement
			) VALUES (
				(SELECT stream_id
					FROM pending_objects WHERE
						project_id   = $12 AND
						bucket_name  = $13 AND
						object_key   = $14 AND
						stream_id    = $15
				), $1, $2,
				$3, $4, $5,
				$6, $7, $8, $9,
				$10,
				$11,
				$16
			)
			ON CONFLICT(stream_id, position)
			DO UPDATE SET
				expires_at = $2,
				root_piece_id = $3, encrypted_key_nonce = $4, encrypted_key = $5,
				encrypted_size = $6, plain_offset = $7, plain_size = $8, encrypted_etag = $9,
				redundancy = $10,
				remote_alias_pieces = $11,
				placement = $16
		`, opts.Position, opts.ExpiresAt,
			opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
			opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
			redundancyScheme{&opts.Redundancy},
			aliasPieces,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
			opts.Placement,
		)
	} else {
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				redundancy,
				remote_alias_pieces,
				placement
			) VALUES (
				(SELECT stream_id
					FROM objects WHERE
						project_id   = $12 AND
						bucket_name  = $13 AND
						object_key   = $14 AND
						version      = $15 AND
						stream_id    = $16 AND
						status       = `+pendingStatus+
			`	), $1, $2,
				$3, $4, $5,
				$6, $7, $8, $9,
				$10,
				$11,
				$17
			)
			ON CONFLICT(stream_id, position)
			DO UPDATE SET
				expires_at = $2,
				root_piece_id = $3, encrypted_key_nonce = $4, encrypted_key = $5,
				encrypted_size = $6, plain_offset = $7, plain_size = $8, encrypted_etag = $9,
				redundancy = $10,
				remote_alias_pieces = $11,
				placement = $17
			`, opts.Position, opts.ExpiresAt,
			opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
			opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
			redundancyScheme{&opts.Redundancy},
			aliasPieces,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
			opts.Placement,
		)
	}
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
		return Error.New("unable to insert segment: %w", err)
	}

	mon.Meter("segment_commit").Mark(1)
	mon.IntVal("segment_commit_encrypted_size").Observe(int64(opts.EncryptedSize))

	return nil
}

// CommitInlineSegment contains all necessary information about the segment.
type CommitInlineSegment struct {
	ObjectStream

	Position SegmentPosition

	ExpiresAt *time.Time

	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	PlainOffset   int64 // offset in the original data stream
	PlainSize     int32 // size before encryption
	EncryptedETag []byte

	InlineData []byte

	UsePendingObjectsTable bool
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

	if opts.UsePendingObjectsTable {
		_, err = db.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				inline_data
			) VALUES (
				(SELECT stream_id
					FROM pending_objects WHERE
						project_id   = $11 AND
						bucket_name  = $12 AND
						object_key   = $13 AND
						stream_id    = $14
				), $1, $2,
				$3, $4, $5,
				$6, $7, $8, $9,
				$10
			)
			ON CONFLICT(stream_id, position)
			DO UPDATE SET
				expires_at = $2,
				root_piece_id = $3, encrypted_key_nonce = $4, encrypted_key = $5,
				encrypted_size = $6, plain_offset = $7, plain_size = $8, encrypted_etag = $9,
				inline_data = $10
		`, opts.Position, opts.ExpiresAt,
			storj.PieceID{}, opts.EncryptedKeyNonce, opts.EncryptedKey,
			len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
			opts.InlineData,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
		)
	} else {
		_, err = db.db.ExecContext(ctx, `
					INSERT INTO segments (
						stream_id, position, expires_at,
						root_piece_id, encrypted_key_nonce, encrypted_key,
						encrypted_size, plain_offset, plain_size, encrypted_etag,
						inline_data
					) VALUES (
						(SELECT stream_id
							FROM objects WHERE
								project_id   = $11 AND
								bucket_name  = $12 AND
								object_key   = $13 AND
								version      = $14 AND
								stream_id    = $15 AND
								status       = `+pendingStatus+
			`	), $1, $2,
						$3, $4, $5,
						$6, $7, $8, $9,
						$10
					)
					ON CONFLICT(stream_id, position)
					DO UPDATE SET
						expires_at = $2,
						root_piece_id = $3, encrypted_key_nonce = $4, encrypted_key = $5,
						encrypted_size = $6, plain_offset = $7, plain_size = $8, encrypted_etag = $9,
						inline_data = $10
				`, opts.Position, opts.ExpiresAt,
			storj.PieceID{}, opts.EncryptedKeyNonce, opts.EncryptedKey,
			len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
			opts.InlineData,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
		)
	}
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
		return Error.New("unable to insert segment: %w", err)
	}

	mon.Meter("segment_commit").Mark(1)
	mon.IntVal("segment_commit_encrypted_size").Observe(int64(len(opts.InlineData)))

	return nil
}

// CommitObject contains arguments necessary for committing an object.
type CommitObject struct {
	ObjectStream

	Encryption storj.EncryptionParameters

	// this flag controls if we want to set metadata fields with CommitObject
	// it's possible to set metadata with BeginObject request so we need to
	// be explicit if we would like to set it with CommitObject which will
	// override any existing metadata.
	OverrideEncryptedMetadata     bool
	EncryptedMetadata             []byte // optional
	EncryptedMetadataNonce        []byte // optional
	EncryptedMetadataEncryptedKey []byte // optional

	DisallowDelete bool

	UsePendingObjectsTable bool
}

// Verify verifies reqest fields.
func (c *CommitObject) Verify() error {
	if err := c.ObjectStream.Verify(); err != nil {
		return err
	}

	if c.Encryption.CipherSuite != storj.EncUnspecified && c.Encryption.BlockSize <= 0 {
		return ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	}

	if c.OverrideEncryptedMetadata {
		if c.EncryptedMetadata == nil && (c.EncryptedMetadataNonce != nil || c.EncryptedMetadataEncryptedKey != nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set")
		} else if c.EncryptedMetadata != nil && (c.EncryptedMetadataNonce == nil || c.EncryptedMetadataEncryptedKey == nil) {
			return ErrInvalidRequest.New("EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set")
		}
	}
	return nil
}

// CommitObject adds a pending object to the database. If another committed object is under target location
// it will be deleted.
func (db *DB) CommitObject(ctx context.Context, opts CommitObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		segments, err := fetchSegmentsForCommit(ctx, tx, opts.StreamID)
		if err != nil {
			return Error.New("failed to fetch segments: %w", err)
		}

		if err = db.validateParts(segments); err != nil {
			return err
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

		args := []interface{}{
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
			len(segments),
			totalPlainSize,
			totalEncryptedSize,
			fixedSegmentSize,
			encryptionParameters{&opts.Encryption},
		}

		versionsToDelete := []Version{}
		if err := withRows(tx.QueryContext(ctx, `
			SELECT version
			FROM objects
			WHERE
				project_id   = $1 AND
				bucket_name  = $2 AND
				object_key   = $3 AND
				status       = `+committedStatus,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var version Version
				if err := rows.Scan(&version); err != nil {
					return Error.New("failed to scan previous object: %w", err)
				}

				versionsToDelete = append(versionsToDelete, version)
			}
			return nil
		}); err != nil {
			return Error.New("failed to find previous objects: %w", err)
		}

		if len(versionsToDelete) > 1 {
			db.log.Warn("object with multiple committed versions were found!",
				zap.Stringer("Project ID", opts.ProjectID), zap.String("Bucket Name", opts.BucketName),
				zap.ByteString("Object Key", []byte(opts.ObjectKey)), zap.Int("deleted", len(versionsToDelete)))

			mon.Meter("multiple_committed_versions").Mark(1)
		}

		if len(versionsToDelete) != 0 && opts.DisallowDelete {
			return ErrPermissionDenied.New("no permissions to delete existing object")
		}

		if opts.UsePendingObjectsTable {
			opts.Version = 1

			// we remove from deletion list object with version 1 as we will
			// override/update instead deleting and inserting new one.
			index := slices.Index(versionsToDelete, opts.Version)
			if index != -1 {
				versionsToDelete = slices.Delete(versionsToDelete, index, index+1)
			}

			args = append(args,
				opts.EncryptedMetadataNonce,
				opts.EncryptedMetadata,
				opts.EncryptedMetadataEncryptedKey,
				opts.OverrideEncryptedMetadata,
			)

			err = tx.QueryRowContext(ctx, `
				WITH delete_pending_object AS (
					DELETE FROM pending_objects WHERE
						project_id   = $1 AND
						bucket_name  = $2 AND
						object_key   = $3 AND
						stream_id    = $5
					RETURNING
						project_id, bucket_name, object_key, $4::INT4 as version, stream_id,
						`+committedStatus+` as status, $6::INT4 as segment_count, $7::INT8 as total_plain_size, $8::INT8 as total_encrypted_size, $9::INT4 as fixed_segment_size,
						NULL::timestamp as zombie_deletion_deadline,
						CASE
							WHEN encryption = 0 AND $10 <> 0 THEN $10
							WHEN encryption = 0 AND $10 = 0 THEN NULL
							ELSE encryption
						END as
						encryption,
						encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
				), object_to_commit AS (
					SELECT
						project_id, bucket_name, object_key, version, stream_id,
						status, segment_count, total_plain_size, total_encrypted_size, fixed_segment_size,
						zombie_deletion_deadline, encryption,
						CASE
							WHEN $14::BOOL = true THEN $11
							ELSE delete_pending_object.encrypted_metadata_nonce
						END as
						encrypted_metadata_nonce,
						CASE
							WHEN $14::BOOL = true THEN $12
							ELSE delete_pending_object.encrypted_metadata
						END as
						encrypted_metadata,
						CASE
							WHEN $14::BOOL = true THEN $13
							ELSE delete_pending_object.encrypted_metadata_encrypted_key
						END as
						encrypted_metadata_encrypted_key
					FROM delete_pending_object
				)
				INSERT INTO objects (
					project_id, bucket_name, object_key, version, stream_id,
					status, segment_count, total_plain_size, total_encrypted_size, fixed_segment_size, zombie_deletion_deadline,
					encryption,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
				)
				SELECT * FROM object_to_commit
				ON CONFLICT (project_id, bucket_name, object_key, version)
				DO UPDATE SET (
					stream_id,
					status, segment_count, total_plain_size, total_encrypted_size, fixed_segment_size, zombie_deletion_deadline,
					encryption,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
				) = (SELECT
						stream_id,
						status, segment_count, total_plain_size, total_encrypted_size, fixed_segment_size, zombie_deletion_deadline,
						encryption,
						encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
					FROM object_to_commit)
				RETURNING
					created_at, expires_at,
					encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_metadata_nonce,
					encryption
			`, args...).Scan(
				&object.CreatedAt, &object.ExpiresAt,
				&object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedMetadataNonce,
				encryptionParameters{&object.Encryption},
			)
		} else {
			metadataColumns := ""
			if opts.OverrideEncryptedMetadata {
				args = append(args,
					opts.EncryptedMetadataNonce,
					opts.EncryptedMetadata,
					opts.EncryptedMetadataEncryptedKey,
				)
				metadataColumns = `,
					encrypted_metadata_nonce         = $11,
					encrypted_metadata               = $12,
					encrypted_metadata_encrypted_key = $13
				`
			}
			err = tx.QueryRowContext(ctx, `
				UPDATE objects SET
					status =`+committedStatus+`,
					segment_count = $6,

					total_plain_size     = $7,
					total_encrypted_size = $8,
					fixed_segment_size   = $9,
					zombie_deletion_deadline = NULL,

					-- TODO should we allow to override existing encryption parameters or return error if don't match with opts?
					encryption = CASE
						WHEN objects.encryption = 0 AND $10 <> 0 THEN $10
						WHEN objects.encryption = 0 AND $10 = 0 THEN NULL
						ELSE objects.encryption
					END
					`+metadataColumns+`
				WHERE
					project_id   = $1 AND
					bucket_name  = $2 AND
					object_key   = $3 AND
					version      = $4 AND
					stream_id    = $5 AND
					status       = `+pendingStatus+`
				RETURNING
					created_at, expires_at,
					encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_metadata_nonce,
					encryption
				`, args...).Scan(
				&object.CreatedAt, &object.ExpiresAt,
				&object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedMetadataNonce,
				encryptionParameters{&object.Encryption},
			)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
			} else if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
				// TODO maybe we should check message if 'encryption' label is there
				return ErrInvalidRequest.New("Encryption is missing")
			}
			return Error.New("failed to update object: %w", err)
		}

		for _, version := range versionsToDelete {
			_, err := db.deleteObjectExactVersion(ctx, DeleteObjectExactVersion{
				ObjectLocation: ObjectLocation{
					ProjectID:  opts.ProjectID,
					BucketName: opts.BucketName,
					ObjectKey:  opts.ObjectKey,
				},
				Version: version,
			}, tx)
			if err != nil {
				return Error.New("failed to delete existing object: %w", err)
			}
		}

		object.StreamID = opts.StreamID
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		object.Version = opts.Version
		object.Status = Committed
		object.SegmentCount = int32(len(segments))
		object.TotalPlainSize = totalPlainSize
		object.TotalEncryptedSize = totalEncryptedSize
		object.FixedSegmentSize = fixedSegmentSize
		return nil
	})
	if err != nil {
		return Object{}, err
	}

	mon.Meter("object_commit").Mark(1)
	mon.IntVal("object_commit_segments").Observe(int64(object.SegmentCount))
	mon.IntVal("object_commit_encrypted_size").Observe(object.TotalEncryptedSize)

	return object, nil
}

func (db *DB) validateParts(segments []segmentInfoForCommit) error {
	partSize := make(map[uint32]memory.Size)

	var lastPart uint32
	for _, segment := range segments {
		partSize[segment.Position.Part] += memory.Size(segment.PlainSize)
		if lastPart < segment.Position.Part {
			lastPart = segment.Position.Part
		}
	}

	if len(partSize) > db.config.MaxNumberOfParts {
		return Error.New("exceeded maximum number of parts: %d", db.config.MaxNumberOfParts)
	}

	for part, size := range partSize {
		// Last part has no minimum size.
		if part == lastPart {
			continue
		}

		if size < db.config.MinPartSize {
			return Error.New("size of part number %d is below minimum threshold, got: %s, min: %s", part, size, db.config.MinPartSize)
		}
	}

	return nil
}
