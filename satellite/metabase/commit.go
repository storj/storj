// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	pgxerrcode "github.com/jackc/pgerrcode"
	spanner "github.com/storj/exp-spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

// we need to disable PlainSize validation for old uplinks.
const validatePlainSize = false

const defaultZombieDeletionPeriod = 24 * time.Hour

var (
	// ErrObjectNotFound is used to indicate that the object does not exist.
	ErrObjectNotFound = errs.Class("object not found")
	// ErrInvalidRequest is used to indicate invalid requests.
	ErrInvalidRequest = errs.Class("metabase: invalid request")
	// ErrFailedPrecondition is used to indicate that some conditions in the request has failed.
	ErrFailedPrecondition = errs.Class("metabase: failed precondition")
	// ErrConflict is used to indicate conflict with the request.
	ErrConflict = errs.Class("metabase: conflict")
)

type commitObjectTransactionAdapter interface {
	updateSegmentOffsets(ctx context.Context, streamID uuid.UUID, updates []segmentToCommit) (err error)
	finalizeObjectCommit(ctx context.Context, opts CommitObject, nextStatus ObjectStatus, nextVersion Version, finalSegments []segmentInfoForCommit, totalPlainSize int64, totalEncryptedSize int64, fixedSegmentSize int32, object *Object) error

	precommitTransactionAdapter
}

// BeginObjectNextVersion contains arguments necessary for starting an object upload.
type BeginObjectNextVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedMetadata             []byte // optional
	EncryptedMetadataNonce        []byte // optional
	EncryptedMetadataEncryptedKey []byte // optional

	Encryption storj.EncryptionParameters
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

	err = db.ChooseAdapter(opts.ProjectID).BeginObjectNextVersion(ctx, opts, &object)
	if err != nil {
		return Object{}, Error.New("unable to insert object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginObjectNextVersion implements Adapter.
func (p *PostgresAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	return p.db.QueryRowContext(ctx, `
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
						WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
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
	).Scan(&object.Status, &object.Version, &object.CreatedAt)
}

// BeginObjectNextVersion implements Adapter.
func (s *SpannerAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	_, err := s.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		enc, err := encryptionParameters{&opts.Encryption}.Value()
		if err != nil {
			return Error.Wrap(err)
		}

		stmt := spanner.Statement{
			SQL: `INSERT objects (
					project_id, bucket_name, object_key, version, stream_id,
					expires_at, encryption,
					zombie_deletion_deadline,
					encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key)
				  VALUES(
                  	@project_id, @bucket_name, @object_key,
					coalesce(
						(SELECT version + 1
						FROM objects
						WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
						ORDER BY version DESC
						LIMIT 1)
					,1),
					@stream_id, @expires_at,
					@encryption, @zombie_deletion_deadline,
					@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key)
                  THEN RETURN status,version,created_at`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID.Bytes(),
				"bucket_name":                      opts.BucketName,
				"object_key":                       opts.ObjectKey,
				"stream_id":                        opts.StreamID.Bytes(),
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       enc,
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
			},
		}
		updateIter := txn.Query(ctx, stmt)
		defer updateIter.Stop()
		for {
			row, err := updateIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return Error.Wrap(err)
			}
			if err := row.Columns(&object.Status, &object.Version, &object.CreatedAt); err != nil {
				return Error.Wrap(err)
			}
		}
		return nil
	})
	return err
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

// TestingBeginObjectExactVersion adds a pending object to the database, with specific version.
func (db *DB) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Object, err error) {
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

	err = db.ChooseAdapter(opts.ProjectID).TestingBeginObjectExactVersion(ctx, opts, &object)
	if err != nil {
		if ErrObjectAlreadyExists.Has(err) {
			return Object{}, err
		}
		return Object{}, Error.New("unable to commit object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// TestingBeginObjectExactVersion implements Adapter.
func (p *PostgresAdapter) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	err := p.db.QueryRowContext(ctx, `
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
			return Error.Wrap(ErrObjectAlreadyExists.New(""))
		}
	}
	return err
}

// TestingBeginObjectExactVersion implements Adapter.
func (s *SpannerAdapter) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	_, err := s.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		stmt := spanner.Statement{
			SQL: `INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
			) VALUES (
				@project_id, @bucket_name, @object_key, @version, @stream_id,
				@expires_at, @encryption,
				@zombie_deletion_deadline,
				@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key
			) THEN RETURN status, created_at`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID,
				"bucket_name":                      opts.BucketName,
				"object_key":                       opts.ObjectKey,
				"version":                          opts.Version,
				"stream_id":                        opts.StreamID,
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       &encryptionParameters{&opts.Encryption},
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
			},
		}
		updateIter := txn.Query(ctx, stmt)
		defer updateIter.Stop()

		row, err := updateIter.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return Error.New("no status returned for inserted object??")
			}
			if errCode := spanner.ErrCode(err); errCode == codes.AlreadyExists {
				return Error.Wrap(ErrObjectAlreadyExists.New(""))
			}
			return Error.Wrap(err)
		}
		if err := row.Columns(&object.Status, &object.CreatedAt); err != nil {
			return Error.Wrap(err)
		}
		return nil
	})
	return err
}

// BeginSegment contains options to verify, whether a new segment upload can be started.
type BeginSegment struct {
	ObjectStream

	Position SegmentPosition

	// TODO: unused field, can remove
	RootPieceID storj.PieceID

	Pieces Pieces

	ObjectExistsChecked bool
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

	if !opts.ObjectExistsChecked {
		// NOTE: Find a way to safely remove this. This isn't strictly necessary,
		// since we can also fail this in CommitSegment.
		// We should prevent creating segments for non-partial objects.

		// Verify that object exists and is partial.
		exists, err := db.ChooseAdapter(opts.ProjectID).PendingObjectExists(ctx, opts)
		if err != nil {
			return Error.New("unable to query object status: %w", err)
		}
		if !exists {
			return ErrPendingObjectMissing.New("")
		}
	}

	mon.Meter("segment_begin").Mark(1)

	return nil
}

// PendingObjectExists checks whether an object already exists.
func (p *PostgresAdapter) PendingObjectExists(ctx context.Context, opts BeginSegment) (exists bool, err error) {
	err = p.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM objects
			WHERE (project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
				status = `+statusPending+`
		)`,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID).Scan(&exists)
	return exists, err
}

// PendingObjectExists checks whether an object already exists.
func (s *SpannerAdapter) PendingObjectExists(ctx context.Context, opts BeginSegment) (exists bool, err error) {
	result := s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT EXISTS (
				SELECT 1
				FROM objects
				WHERE
					project_id      = @project_id
					AND bucket_name = @bucket_name
					AND object_key  = @object_key
					AND version     = @version
					AND stream_id   = @stream_id
					AND status      = ` + statusPending + `
			)
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
			"stream_id":   opts.StreamID,
		},
	})
	defer result.Stop()
	for {
		row, err := result.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return false, Error.Wrap(err)
		}
		if err := row.Columns(&exists); err != nil {
			return false, Error.Wrap(err)
		}
	}
	return exists, nil
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

	err = db.ChooseAdapter(opts.ProjectID).CommitPendingObjectSegment(ctx, opts, aliasPieces)
	if err != nil {
		if ErrPendingObjectMissing.Has(err) {
			return err
		}
		return Error.New("unable to insert segment: %w", err)
	}

	mon.Meter("segment_commit").Mark(1)
	mon.IntVal("segment_commit_encrypted_size").Observe(int64(opts.EncryptedSize))

	return nil
}

// CommitPendingObjectSegment commits segment to the database.
func (p *PostgresAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Verify that object exists and is partial.
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO segments (
			stream_id, position, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size, encrypted_etag,
			redundancy,
			remote_alias_pieces,
			placement
		) VALUES (
			(
				SELECT stream_id
				FROM objects
				WHERE (project_id, bucket_name, object_key, version, stream_id) = ($12, $13, $14, $15, $16) AND
					status = `+statusPending+`
			), $1, $2,
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
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}
	return err
}

// CommitPendingObjectSegment commits segment to the database.
func (p *CockroachAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Verify that object exists and is partial.
	_, err = p.db.ExecContext(ctx, `
			UPSERT INTO segments (
				stream_id, position,
				expires_at, root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				redundancy,
				remote_alias_pieces,
				placement
			) VALUES (
				(
					SELECT stream_id
					FROM objects
					WHERE (project_id, bucket_name, object_key, version, stream_id) = ($12, $13, $14, $15, $16) AND
						status = `+statusPending+`
				), $1, $2,
				$3, $4, $5,
				$6, $7, $8, $9,
				$10,
				$11,
				$17
			)`, opts.Position, opts.ExpiresAt,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		redundancyScheme{&opts.Redundancy},
		aliasPieces,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
		opts.Placement,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}
	return err
}

// CommitPendingObjectSegment commits segment to the database.
func (s *SpannerAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) error {
	panic("implement me")
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

	err = db.ChooseAdapter(opts.ProjectID).CommitInlineSegment(ctx, opts)
	if err != nil {
		if ErrPendingObjectMissing.Has(err) {
			return err
		}
		return Error.New("unable to insert segment: %w", err)
	}
	mon.Meter("segment_commit").Mark(1)
	mon.IntVal("segment_commit_encrypted_size").Observe(int64(len(opts.InlineData)))

	return nil
}

// CommitInlineSegment commits inline segment to the database.
func (p *PostgresAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	_, err = p.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				inline_data
			) VALUES (
				(
					SELECT stream_id
					FROM objects
					WHERE (project_id, bucket_name, object_key, version, stream_id) = ($11, $12, $13, $14, $15) AND
						status = `+statusPending+`
				),
				$1, $2,
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
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}

	return Error.Wrap(err)
}

// CommitInlineSegment commits inline segment to the database.
func (p *CockroachAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	return txutil.WithTx(ctx, p.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT stream_id
			FROM objects
			WHERE
			(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5)
			AND status = `+statusPending+`
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPendingObjectMissing.New("")
			}
			return Error.Wrap(err)
		}

		if !rows.Next() {
			return ErrPendingObjectMissing.New("")
		}

		if err := errs.Combine(rows.Err(), rows.Close()); err != nil {
			return Error.Wrap(err)
		}

		_, err = tx.ExecContext(ctx, `
			UPSERT INTO segments (
				stream_id, position, expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				inline_data
			) VALUES (
				$11,
				$1, $2,
				$3, $4, $5,
				$6, $7, $8, $9,
				$10
			)
		`, opts.Position, opts.ExpiresAt,
			storj.PieceID{}, opts.EncryptedKeyNonce, opts.EncryptedKey,
			len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
			opts.InlineData,
			opts.StreamID,
		)
		if err != nil {
			if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
				return ErrPendingObjectMissing.New("")
			}
		}
		return err
	})
}

// CommitInlineSegment commits inline segment to the database.
func (s *SpannerAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) error {
	panic("implement me")
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

	// Versioned indicates whether an object is allowed to have multiple versions.
	Versioned bool
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

// WithTx provides a TransactionAdapter for the context of a database transaction.
func (p *PostgresAdapter) WithTx(ctx context.Context, f func(context.Context, TransactionAdapter) error) error {
	return txutil.WithTx(ctx, p.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		txAdapter := &postgresTransactionAdapter{postgresAdapter: p, tx: tx}
		return f(ctx, txAdapter)
	})
}

// WithTx provides a TransactionAdapter for the context of a database transaction.
func (s *SpannerAdapter) WithTx(ctx context.Context, f func(context.Context, TransactionAdapter) error) error {
	panic("implement me")
}

// CommitObject adds a pending object to the database. If another committed object is under target location
// it will be deleted.
func (db *DB) CommitObject(ctx context.Context, opts CommitObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	var precommit precommitConstraintResult
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, func(ctx context.Context, adapter TransactionAdapter) error {
		segments, err := adapter.fetchSegmentsForCommit(ctx, opts.StreamID)
		if err != nil {
			return Error.New("failed to fetch segments: %w", err)
		}

		if err = db.validateParts(segments); err != nil {
			return err
		}

		finalSegments := convertToFinalSegments(segments)
		if err := adapter.updateSegmentOffsets(ctx, opts.StreamID, finalSegments); err != nil {
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

		nextStatus := committedWhereVersioned(opts.Versioned)

		precommit, err = db.precommitConstraint(ctx, precommitConstraint{
			Location:       opts.Location(),
			Versioned:      opts.Versioned,
			DisallowDelete: opts.DisallowDelete,
		}, adapter)
		if err != nil {
			return err
		}

		nextVersion := opts.Version
		if nextVersion < precommit.HighestVersion {
			nextVersion = precommit.HighestVersion + 1
		}

		err = adapter.finalizeObjectCommit(ctx, opts, nextStatus, nextVersion, segments, totalPlainSize, totalEncryptedSize, fixedSegmentSize, &object)
		if err != nil {
			return err
		}

		object.StreamID = opts.StreamID
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		object.Version = nextVersion
		object.Status = nextStatus
		object.SegmentCount = int32(len(segments))
		object.TotalPlainSize = totalPlainSize
		object.TotalEncryptedSize = totalEncryptedSize
		object.FixedSegmentSize = fixedSegmentSize
		return nil
	})
	if err != nil {
		return Object{}, err
	}

	precommit.submitMetrics()

	mon.Meter("object_commit").Mark(1)
	mon.IntVal("object_commit_segments").Observe(int64(object.SegmentCount))
	mon.IntVal("object_commit_encrypted_size").Observe(object.TotalEncryptedSize)

	return object, nil
}

func (ptx *postgresTransactionAdapter) finalizeObjectCommit(ctx context.Context, opts CommitObject, nextStatus ObjectStatus, nextVersion Version, finalSegments []segmentInfoForCommit, totalPlainSize int64, totalEncryptedSize int64, fixedSegmentSize int32, object *Object) (err error) {
	defer mon.Task()(&ctx)(&err)

	args := []interface{}{
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
		nextStatus,
		len(finalSegments),
		totalPlainSize,
		totalEncryptedSize,
		fixedSegmentSize,
		encryptionParameters{&opts.Encryption},
	}

	args = append(args, nextVersion)

	metadataColumns := ""
	if opts.OverrideEncryptedMetadata {
		args = append(args,
			opts.EncryptedMetadataNonce,
			opts.EncryptedMetadata,
			opts.EncryptedMetadataEncryptedKey,
		)
		metadataColumns = `,
				encrypted_metadata_nonce         = $13,
				encrypted_metadata               = $14,
				encrypted_metadata_encrypted_key = $15
			`
	}
	err = ptx.tx.QueryRowContext(ctx, `
			UPDATE objects SET
				version = $12,
				status = $6,
				segment_count = $7,

				total_plain_size     = $8,
				total_encrypted_size = $9,
				fixed_segment_size   = $10,
				zombie_deletion_deadline = NULL,

				-- TODO should we allow to override existing encryption parameters or return error if don't match with opts?
				encryption = CASE
					WHEN objects.encryption = 0 AND $11 <> 0 THEN $11
					WHEN objects.encryption = 0 AND $11 = 0 THEN NULL
					ELSE objects.encryption
				END
				`+metadataColumns+`
			WHERE (project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
				status       = `+statusPending+`
			RETURNING
				created_at, expires_at,
				encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_metadata_nonce,
				encryption
			`, args...).Scan(
		&object.CreatedAt, &object.ExpiresAt,
		&object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedMetadataNonce,
		encryptionParameters{&object.Encryption},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
		} else if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			// TODO maybe we should check message if 'encryption' label is there
			return ErrInvalidRequest.New("Encryption is missing")
		}
		return Error.New("failed to update object: %w", err)
	}
	return nil
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
		return ErrFailedPrecondition.New("exceeded maximum number of parts: %d", db.config.MaxNumberOfParts)
	}

	for part, size := range partSize {
		// Last part has no minimum size.
		if part == lastPart {
			continue
		}

		if size < db.config.MinPartSize {
			return ErrFailedPrecondition.New("size of part number %d is below minimum threshold, got: %s, min: %s", part, size, db.config.MinPartSize)
		}
	}

	return nil
}
