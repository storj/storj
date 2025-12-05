// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

// ValidatePlainSize determines whether we disable PlainSize validation for old uplinks.
const ValidatePlainSize = false

const (
	defaultZombieDeletionPeriod           = 24 * time.Hour
	defaultZombieDeletionCopyObjectPeriod = 1 * time.Hour
)

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
	finalizeObjectCommit(ctx context.Context, opts finalizeObjectCommit) (err error)

	precommitInsertObject(ctx context.Context, object *Object, segments []*Segment) (err error)

	precommitInsertOrUpdateObject(ctx context.Context, object *Object, segments []*Segment) (err error)

	// precommitDeleteExactObject deletes the exact object and segments.
	// It does not check object lock constraints.
	precommitDeleteExactObject(ctx context.Context, stream ObjectStream) (err error)
	// precommitDeleteExactSegments deletes the segments under specific stream id.
	// It does not check object lock constraints.
	precommitDeleteExactSegments(ctx context.Context, streamID uuid.UUID) (err error)

	precommitTransactionAdapter
}

// BeginObjectNextVersion contains arguments necessary for starting an object upload.
type BeginObjectNextVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedUserData
	Encryption storj.EncryptionParameters

	Retention Retention // optional
	LegalHold bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify verifies get object request fields.
func (opts *BeginObjectNextVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version != NextVersion {
		return ErrInvalidRequest.New("Version should be metabase.NextVersion")
	}

	err := opts.EncryptedUserData.Verify()
	if err != nil {
		return err
	}

	if err := opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if opts.ExpiresAt != nil {
		switch {
		case opts.Retention.Enabled():
			return ErrInvalidRequest.New("ExpiresAt must not be set if Retention is set")
		case opts.LegalHold:
			return ErrInvalidRequest.New("ExpiresAt must not be set if LegalHold is set")
		}
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
		Status:                 DefaultStatus,
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		EncryptedUserData:      opts.EncryptedUserData,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
		Retention:              opts.Retention,
		LegalHold:              opts.LegalHold,
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
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
				retention_mode, retain_until
			) VALUES (
				$1, $2, $3, `+p.generateVersion()+`,
				$4, $5, $6,
				$7,
				$8, $9, $10, $11,
				$12, $13
			)
			RETURNING version, created_at
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.StreamID,
		opts.ExpiresAt, opts.Encryption,
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey, opts.EncryptedETag,
		lockModeWrapper{
			retentionMode: &opts.Retention.Mode,
			legalHold:     &opts.LegalHold,
		}, timeWrapper{&opts.Retention.RetainUntil},
	).Scan(&object.Version, &object.CreatedAt)
}

// BeginObjectNextVersion implements Adapter.
func (s *SpannerAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	_, err := s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		return Error.Wrap(txn.Query(ctx, spanner.Statement{
			SQL: `INSERT objects (
					project_id, bucket_name, object_key, version, stream_id,
					expires_at, encryption,
					zombie_deletion_deadline,
					encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
					retention_mode, retain_until
				) VALUES (
					@project_id, @bucket_name, @object_key,
					` + s.generateVersion() + `,
					@stream_id, @expires_at,
					@encryption, @zombie_deletion_deadline,
					@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key, @encrypted_etag,
					@retention_mode, @retain_until
				)
				THEN RETURN version, created_at`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID.Bytes(),
				"bucket_name":                      opts.BucketName,
				"object_key":                       opts.ObjectKey,
				"stream_id":                        opts.StreamID.Bytes(),
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       opts.Encryption,
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
				"encrypted_etag":                   opts.EncryptedETag,
				"retention_mode": lockModeWrapper{
					retentionMode: &opts.Retention.Mode,
					legalHold:     &opts.LegalHold,
				},
				"retain_until": timeWrapper{&opts.Retention.RetainUntil},
			},
		}).Do(func(row *spanner.Row) error {
			return Error.Wrap(row.Columns(&object.Version, &object.CreatedAt))
		}))
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              "begin-object-next-version",
		ExcludeTxnFromChangeStreams: true,
	})
	return err
}

// BeginObjectExactVersion contains arguments necessary for starting an object upload.
type BeginObjectExactVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedUserData
	Encryption storj.EncryptionParameters

	Retention Retention // optional
	LegalHold bool

	// TestingBypassVerify makes the (*DB).TestingBeginObjectExactVersion method skip
	// validation of this struct's fields. This is useful for inserting intentionally
	// malformed or unexpected data into the database and testing that we handle it properly.
	TestingBypassVerify bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify verifies get object reqest fields.
func (opts *BeginObjectExactVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version == NextVersion {
		return ErrInvalidRequest.New("Version should not be metabase.NextVersion")
	}

	err := opts.EncryptedUserData.Verify()
	if err != nil {
		return err
	}

	if err := opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if opts.ExpiresAt != nil {
		switch {
		case opts.Retention.Enabled():
			return ErrInvalidRequest.New("ExpiresAt must not be set if Retention is set")
		case opts.LegalHold:
			return ErrInvalidRequest.New("ExpiresAt must not be set if LegalHold is set")
		}
	}

	return nil
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (db *DB) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if !opts.TestingBypassVerify {
		if err := opts.Verify(); err != nil {
			return Object{}, err
		}
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
		Status:                 DefaultStatus,
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		EncryptedUserData:      opts.EncryptedUserData,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
		Retention:              opts.Retention,
		LegalHold:              opts.LegalHold,
	}

	err = db.ChooseAdapter(opts.ProjectID).BeginObjectExactVersion(ctx, opts, &object)
	if err != nil {
		if ErrObjectAlreadyExists.Has(err) {
			return Object{}, err
		}
		return Object{}, Error.New("unable to commit object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginObjectExactVersion implements Adapter.
func (p *PostgresAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline,
			encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
			retention_mode, retain_until
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8,
			$9, $10, $11, $12,
			$13, $14
		)
		RETURNING created_at
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID,
		opts.ExpiresAt, opts.Encryption,
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey, opts.EncryptedETag,
		lockModeWrapper{
			retentionMode: &opts.Retention.Mode,
			legalHold:     &opts.LegalHold,
		}, timeWrapper{&opts.Retention.RetainUntil},
	).Scan(
		&object.CreatedAt,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return Error.Wrap(ErrObjectAlreadyExists.New(""))
		}
	}
	return err
}

// BeginObjectExactVersion implements Adapter.
func (s *SpannerAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	_, err := s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		err := txn.Query(ctx, spanner.Statement{
			SQL: `INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
				retention_mode, retain_until
			) VALUES (
				@project_id, @bucket_name, @object_key, @version, @stream_id,
				@expires_at, @encryption,
				@zombie_deletion_deadline,
				@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key, @encrypted_etag,
				@retention_mode, @retain_until
			) THEN RETURN created_at`,
			Params: map[string]interface{}{
				"project_id":                       opts.ProjectID,
				"bucket_name":                      opts.BucketName,
				"object_key":                       opts.ObjectKey,
				"version":                          opts.Version,
				"stream_id":                        opts.StreamID,
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       opts.Encryption,
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
				"encrypted_etag":                   opts.EncryptedETag,
				"retention_mode": lockModeWrapper{
					retentionMode: &opts.Retention.Mode,
					legalHold:     &opts.LegalHold,
				},
				"retain_until": timeWrapper{&opts.Retention.RetainUntil},
			},
		}).Do(func(row *spanner.Row) error {
			return Error.Wrap(row.Columns(&object.CreatedAt))
		})
		if err != nil {
			if errCode := spanner.ErrCode(err); errCode == codes.AlreadyExists {
				return Error.Wrap(ErrObjectAlreadyExists.New(""))
			}
			return Error.Wrap(err)
		}

		return nil
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              "begin-object-exact-version",
		ExcludeTxnFromChangeStreams: true,
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
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID).Scan(&exists)
	return exists, err
}

// PendingObjectExists checks whether an object already exists.
func (s *SpannerAdapter) PendingObjectExists(ctx context.Context, opts BeginSegment) (exists bool, err error) {
	err = s.client.Single().QueryWithOptions(ctx, spanner.Statement{
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
	}, spanner.QueryOptions{RequestTag: "pending-object-exists"}).Do(func(row *spanner.Row) error {
		return Error.Wrap(row.Columns(&exists))
	})
	return exists, Error.Wrap(err)
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

	// supported only by Spanner.
	MaxCommitDelay *time.Duration

	SkipPendingObject bool

	TestingUseMutations bool
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
	case opts.PlainSize <= 0 && ValidatePlainSize:
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

	values := []any{
		opts.StreamID, opts.Position,
		opts.ExpiresAt,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		opts.Redundancy,
		aliasPieces,

		opts.Placement,

		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
	}

	var streamID string
	if !opts.SkipPendingObject {
		streamID = `
			(
				SELECT stream_id
				FROM objects
				WHERE (project_id, bucket_name, object_key, version, stream_id) = ($14, $15, $16, $17, $1) AND
					status = ` + statusPending + `
			)
		`
	} else {
		// When SkipPendingObject=true, check if committed object exists with this stream_id
		streamID = `
			(
				SELECT CASE
					WHEN EXISTS (
						SELECT 1 FROM objects
						WHERE (project_id, bucket_name, object_key, version) = ($14, $15, $16, $17)
							AND stream_id = $1
							AND status IN (` + statusCommittedUnversioned + `, ` + statusCommittedVersioned + `)
					) THEN NULL
					ELSE $1
				END
			)
		`
	}

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
			`+streamID+`, $2,
			$3,
			$4, $5, $6,
			$7, $8, $9, $10,
			$11,
			$12,
			$13
		)
		ON CONFLICT(stream_id, position)
		DO UPDATE SET
			expires_at = $3,
			root_piece_id = $4, encrypted_key_nonce = $5, encrypted_key = $6,
			encrypted_size = $7, plain_offset = $8, plain_size = $9, encrypted_etag = $10,
			redundancy = $11,
			remote_alias_pieces = $12,
			placement = $13,
			-- clear fields in case it was inline segment before
			inline_data = NULL
		`, values...,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}

	return Error.Wrap(err)
}

// CommitPendingObjectSegment commits segment to the database.
func (p *CockroachAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	values := []any{
		opts.StreamID, opts.Position,
		opts.ExpiresAt,
		opts.RootPieceID, opts.EncryptedKeyNonce, opts.EncryptedKey,
		opts.EncryptedSize, opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		opts.Redundancy,
		aliasPieces,

		opts.Placement,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
	}

	var streamID string
	if !opts.SkipPendingObject {
		streamID = `
		(
			SELECT stream_id
			FROM objects
			WHERE (project_id, bucket_name, object_key, version, stream_id) = ($14, $15, $16, $17, $1) AND
				status = ` + statusPending + `
		)
		`
	} else {
		// When SkipPendingObject=true, check if committed object exists with this stream_id
		streamID = `
			(
				SELECT CASE
					WHEN EXISTS (
						SELECT 1 FROM objects
						WHERE (project_id, bucket_name, object_key, version) = ($14, $15, $16, $17)
							AND stream_id = $1
							AND status IN (` + statusCommittedUnversioned + `, ` + statusCommittedVersioned + `)
					) THEN NULL
					ELSE $1
				END
			)
		`
	}

	// Verify that object exists and is partial.
	_, err = p.db.ExecContext(ctx, `
			UPSERT INTO segments (
				stream_id, position,
				expires_at, root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				redundancy,
				remote_alias_pieces,
				placement,
				-- clear fields in case it was inline segment before
				inline_data
			) VALUES (
				`+streamID+`, $2,
				$3,
				$4, $5, $6,
				$7, $8, $9, $10,
				$11,
				$12,
				$13,
				NULL
			)`, values...,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}

	return Error.Wrap(err)
}

// CommitPendingObjectSegment commits segment to the database.
func (s *SpannerAdapter) CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.TestingUseMutations || opts.SkipPendingObject {
		return s.commitPendingObjectSegmentWithMutations(ctx, opts, aliasPieces)
	}

	var numRows int64
	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		stmt := spanner.Statement{
			SQL: `
				INSERT OR UPDATE INTO segments (
					stream_id, position,
					expires_at, root_piece_id, encrypted_key_nonce, encrypted_key,
					encrypted_size, plain_offset, plain_size, encrypted_etag,
					redundancy,
					remote_alias_pieces,
					placement,
					-- clear column in case it was inline segment before
					inline_data
				) VALUES (
					(
						SELECT stream_id
						FROM objects
						WHERE (project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id) AND
							status = ` + statusPending + `
					), @position,
					@expires_at, @root_piece_id, @encrypted_key_nonce, @encrypted_key,
					@encrypted_size, @plain_offset, @plain_size, @encrypted_etag,
					@redundancy,
					@alias_pieces,
					@placement,
					NULL
				)
			`,
			Params: map[string]interface{}{
				"position":            opts.Position,
				"expires_at":          opts.ExpiresAt,
				"root_piece_id":       opts.RootPieceID,
				"encrypted_key_nonce": opts.EncryptedKeyNonce,
				"encrypted_key":       opts.EncryptedKey,
				"encrypted_size":      int64(opts.EncryptedSize),
				"plain_offset":        opts.PlainOffset,
				"plain_size":          int64(opts.PlainSize),
				"encrypted_etag":      opts.EncryptedETag,
				"redundancy":          opts.Redundancy,
				"alias_pieces":        aliasPieces,
				"project_id":          opts.ProjectID,
				"bucket_name":         opts.BucketName,
				"object_key":          opts.ObjectKey,
				"version":             opts.Version,
				"stream_id":           opts.StreamID,
				"placement":           opts.Placement,
			},
		}
		numRows, err = txn.Update(ctx, stmt)
		return err
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              "commit-pending-object-segment",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		if spanner.ErrCode(err) == codes.FailedPrecondition {
			// TODO(spanner) dirty hack to distinguish FailedPrecondition errors.
			// Another issue is that emulator returns different message than real spanner instance.
			if strings.Contains(err.Error(), "column: segments.stream_id") ||
				strings.Contains(err.Error(), "stream_id must not be NULL in table segments") {
				return ErrPendingObjectMissing.New("")
			}
			return ErrFailedPrecondition.Wrap(err)
		}
		return Error.Wrap(err)
	}
	if numRows < 1 {
		return ErrPendingObjectMissing.New("")
	}
	return nil
}

func (s *SpannerAdapter) commitPendingObjectSegmentWithMutations(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	mutation := spanner.InsertOrUpdateMap("segments", map[string]any{
		"stream_id":           opts.StreamID,
		"position":            opts.Position,
		"expires_at":          opts.ExpiresAt,
		"root_piece_id":       opts.RootPieceID,
		"encrypted_key_nonce": opts.EncryptedKeyNonce,
		"encrypted_key":       opts.EncryptedKey,
		"encrypted_size":      int64(opts.EncryptedSize),
		"plain_offset":        opts.PlainOffset,
		"plain_size":          int64(opts.PlainSize),
		"encrypted_etag":      opts.EncryptedETag,
		"redundancy":          opts.Redundancy,
		"remote_alias_pieces": aliasPieces,
		"placement":           opts.Placement,
		"inline_data":         nil, // clear column in case it was inline segment before
	})

	return s.commitSegmentWithMutations(ctx, mutation, internalCommitSegment{
		ObjectStream:      opts.ObjectStream,
		SkipPendingObject: opts.SkipPendingObject,
		MaxCommitDelay:    opts.MaxCommitDelay,
		TransactionTag:    "commit-pending-object-segment-mutations-insert",
	})
}

type internalCommitSegment struct {
	ObjectStream

	SkipPendingObject bool
	MaxCommitDelay    *time.Duration

	TransactionTag string
}

func (s *SpannerAdapter) commitSegmentWithMutations(ctx context.Context, mutation *spanner.Mutation, opts internalCommitSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		row, err := txn.ReadRow(ctx,
			"objects",
			spanner.Key{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version},
			[]string{"stream_id", "status"},
		)
		if err != nil && !errors.Is(err, spanner.ErrRowNotFound) {
			return ErrFailedPrecondition.Wrap(err)
		}

		found := !errors.Is(err, spanner.ErrRowNotFound)
		if found {
			var streamID uuid.UUID
			var status int64
			if err = row.Columns(&streamID, &status); err != nil {
				return Error.Wrap(err)
			}

			if opts.SkipPendingObject {
				// object was already committed
				if streamID == opts.StreamID && (status == int64(CommittedUnversioned) || status == int64(CommittedVersioned)) {
					return ErrPendingObjectMissing.New("")
				}
			} else {
				// pending object must exist
				if streamID != opts.StreamID || status != int64(Pending) {
					return ErrPendingObjectMissing.New("")
				}
			}
		} else if !opts.SkipPendingObject {
			return ErrPendingObjectMissing.New("")
		}

		return errs.Wrap(txn.BufferWrite([]*spanner.Mutation{mutation}))
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              opts.TransactionTag,
		ExcludeTxnFromChangeStreams: true,
	})

	return Error.Wrap(err)
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

	SkipPendingObject bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify verifies commit inline segment reqest fields.
func (opts CommitInlineSegment) Verify() error {
	switch {
	case len(opts.EncryptedKey) == 0:
		return ErrInvalidRequest.New("EncryptedKey missing")
	case len(opts.EncryptedKeyNonce) == 0:
		return ErrInvalidRequest.New("EncryptedKeyNonce missing")
	case opts.PlainSize <= 0 && ValidatePlainSize:
		return ErrInvalidRequest.New("PlainSize negative or zero")
	case opts.PlainOffset < 0:
		return ErrInvalidRequest.New("PlainOffset negative")
	}
	return nil
}

// CommitInlineSegment commits inline segment to the database.
func (db *DB) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if err := opts.Verify(); err != nil {
		return err
	}

	// TODO: do we have a lower limit for inline data?
	// TODO should we move check for max inline segment from metainfo here
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
	values := []any{
		opts.StreamID, opts.Position, opts.ExpiresAt,
		storj.PieceID{},
		opts.EncryptedKeyNonce, opts.EncryptedKey,
		len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		opts.InlineData,

		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
	}

	var streamID string
	if !opts.SkipPendingObject {
		streamID = `
			(
				SELECT stream_id
				FROM objects
				WHERE (project_id, bucket_name, object_key, version, stream_id) = ($12, $13, $14, $15, $1) AND
					status = ` + statusPending + `
			)
		`
	} else {
		// When SkipPendingObject=true, check if committed object exists with this stream_id
		streamID = `
			(
				SELECT CASE
					WHEN EXISTS (
						SELECT 1 FROM objects
						WHERE (project_id, bucket_name, object_key, version) = ($12, $13, $14, $15)
							AND stream_id = $1
							AND status IN (` + statusCommittedUnversioned + `, ` + statusCommittedVersioned + `)
					) THEN NULL
					ELSE $1
				END
			)
		`
	}

	_, err = p.db.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position,
				expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				inline_data
			) VALUES (
				`+streamID+`, $2,
				$3,
				$4, $5, $6,
				$7, $8, $9, $10,
				$11
			)
			ON CONFLICT(stream_id, position)
			DO UPDATE SET
				expires_at = $3,
				root_piece_id = $4, encrypted_key_nonce = $5, encrypted_key = $6,
				encrypted_size = $7, plain_offset = $8, plain_size = $9, encrypted_etag = $10,
				inline_data = $11,
				-- clear columns in case it was remote segment before
				redundancy = 0, remote_alias_pieces = NULL
		`, values...,
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
	values := []any{
		opts.StreamID, opts.Position, opts.ExpiresAt,
		storj.PieceID{},
		opts.EncryptedKeyNonce, opts.EncryptedKey,
		len(opts.InlineData), opts.PlainOffset, opts.PlainSize, opts.EncryptedETag,
		opts.InlineData,
	}

	var streamID string
	if !opts.SkipPendingObject {
		values = append(values, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version)

		streamID = `
			(
				SELECT stream_id
				FROM objects
				WHERE (project_id, bucket_name, object_key, version, stream_id) = ($12, $13, $14, $15, $1) AND
					status = ` + statusPending + `
			)
		`
	} else {
		// When SkipPendingObject=true, check if committed object exists with this stream_id
		values = append(values, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version)

		streamID = `
			(
				SELECT CASE
					WHEN EXISTS (
						SELECT 1 FROM objects
						WHERE (project_id, bucket_name, object_key, version) = ($12, $13, $14, $15)
							AND stream_id = $1
							AND status IN (` + statusCommittedUnversioned + `, ` + statusCommittedVersioned + `)
					) THEN NULL
					ELSE $1
				END
			)
		`
	}

	_, err = p.db.ExecContext(ctx, `
			UPSERT INTO segments (
				stream_id, position,
				expires_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size, encrypted_etag,
				inline_data,
				-- clear columns in case it was remote segment before
				redundancy, remote_alias_pieces
			) VALUES (
				`+streamID+`, $2,
				$3,
				$4, $5, $6,
				$7, $8, $9, $10,
				$11,
				0, NULL
			)
		`, values...,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.NotNullViolation {
			return ErrPendingObjectMissing.New("")
		}
	}

	return Error.Wrap(err)
}

// CommitInlineSegment commits inline segment to the database.
func (s *SpannerAdapter) CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) (err error) {
	if opts.SkipPendingObject {
		mutation := spanner.InsertOrUpdateMap("segments", map[string]any{
			"stream_id":           opts.StreamID,
			"position":            opts.Position,
			"expires_at":          opts.ExpiresAt,
			"root_piece_id":       storj.PieceID{},
			"redundancy":          0,
			"remote_alias_pieces": nil,
			"encrypted_key_nonce": opts.EncryptedKeyNonce,
			"encrypted_key":       opts.EncryptedKey,
			"encrypted_size":      len(opts.InlineData),
			"plain_offset":        opts.PlainOffset,
			"plain_size":          int64(opts.PlainSize),
			"encrypted_etag":      opts.EncryptedETag,
			"inline_data":         opts.InlineData,
		})

		return s.commitSegmentWithMutations(ctx, mutation, internalCommitSegment{
			ObjectStream:      opts.ObjectStream,
			SkipPendingObject: opts.SkipPendingObject,
			MaxCommitDelay:    opts.MaxCommitDelay,
			TransactionTag:    "commit-inline-segment-with-mutation",
		})
	}

	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		_, err := txn.Update(ctx, spanner.Statement{
			SQL: `
				INSERT OR UPDATE INTO segments (
					stream_id, position, expires_at,
					root_piece_id, encrypted_key_nonce, encrypted_key,
					encrypted_size, plain_offset, plain_size, encrypted_etag,
					inline_data,
					-- clear columns in case it was remote segment before
					 redundancy, remote_alias_pieces
				) VALUES (
					(
						SELECT stream_id
						FROM objects
						WHERE (project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id) AND
							status = ` + statusPending + `
					), @position, @expires_at,
					@root_piece_id, @encrypted_key_nonce, @encrypted_key,
					@encrypted_size, @plain_offset, @plain_size, @encrypted_etag,
					@inline_data,
					0, NULL
				)
			`,
			Params: map[string]interface{}{
				"position":            opts.Position,
				"expires_at":          opts.ExpiresAt,
				"root_piece_id":       storj.PieceID{},
				"encrypted_key_nonce": opts.EncryptedKeyNonce,
				"encrypted_key":       opts.EncryptedKey,
				"encrypted_size":      len(opts.InlineData),
				"plain_offset":        opts.PlainOffset,
				"plain_size":          int64(opts.PlainSize),
				"encrypted_etag":      opts.EncryptedETag,
				"inline_data":         opts.InlineData,
				"project_id":          opts.ProjectID.Bytes(),
				"bucket_name":         opts.BucketName,
				"object_key":          opts.ObjectKey,
				"version":             opts.Version,
				"stream_id":           opts.StreamID,
			},
		})
		return Error.Wrap(err)
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              "commit-inline-segment",
		ExcludeTxnFromChangeStreams: true,
	})
	if err != nil {
		if code := spanner.ErrCode(err); code == codes.FailedPrecondition {
			return ErrPendingObjectMissing.New("")
		}
	}
	return Error.Wrap(err)
}

// CommitObject contains arguments necessary for committing an object.
type CommitObject struct {
	ObjectStream

	Encryption storj.EncryptionParameters
	ExpiresAt  *time.Time

	// OverrideEncryptedMedata flag controls if we want to set metadata fields with CommitObject
	// it's possible to set metadata with BeginObject request so we need to
	// be explicit if we would like to set it with CommitObject which will
	// override any existing metadata.
	OverrideEncryptedMetadata bool
	EncryptedUserData

	// Retention and LegalHold are used only for regular uploads (when SkipPendingObject is true).
	// For multipart uploads, these values are retrieved from the pending object in the database.
	Retention Retention // optional
	LegalHold bool

	// TODO: maybe this should use segment ranges rather than individual items
	SpecificSegments bool
	OnlySegments     []SegmentPosition

	DisallowDelete bool

	// Versioned indicates whether an object is allowed to have multiple versions.
	Versioned bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
	TransmitEvent  bool

	// IfNoneMatch is an optional field for conditional writes.
	IfNoneMatch IfNoneMatch

	// SkipPendingObject indicates whether to skip checking for the existence of a pending object.
	// It's used for regular (non-multipart) uploads where we directly commit the object without a prior pending state.
	SkipPendingObject bool
}

// Verify verifies request fields.
func (c *CommitObject) Verify() error {
	if err := c.ObjectStream.Verify(); err != nil {
		return err
	}

	if c.Encryption.CipherSuite != storj.EncUnspecified && c.Encryption.BlockSize <= 0 {
		return ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	}

	if c.OverrideEncryptedMetadata {
		err := c.EncryptedUserData.Verify()
		if err != nil {
			return err
		}
	}

	if err := c.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if c.SpecificSegments {
		if len(c.OnlySegments) == 0 {
			return ErrInvalidRequest.New("no segments specified for commit")
		}

		if err := verifySegmentOrder(c.OnlySegments); err != nil {
			return err
		}
	} else {
		if len(c.OnlySegments) > 0 {
			return ErrInvalidRequest.New("segments specified for commit")
		}
	}

	return c.IfNoneMatch.Verify()
}

// WithTx provides a TransactionAdapter for the context of a database transaction.
func (p *PostgresAdapter) WithTx(ctx context.Context, opts TransactionOptions, f func(context.Context, TransactionAdapter) error) error {
	return txutil.WithTx(ctx, p.db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		txAdapter := &postgresTransactionAdapter{postgresAdapter: p, tx: tx}
		return f(ctx, txAdapter)
	})
}

// WithTx provides a TransactionAdapter for the context of a database transaction.
func (s *SpannerAdapter) WithTx(ctx context.Context, opts TransactionOptions, f func(context.Context, TransactionAdapter) error) error {
	transactionTag := opts.TransactionTag
	if transactionTag == "" {
		transactionTag = "metabase-withtx"
	}
	_, err := s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		txAdapter := &spannerTransactionAdapter{spannerAdapter: s, tx: tx}
		return f(ctx, txAdapter)
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              transactionTag,
		ExcludeTxnFromChangeStreams: !opts.TransmitEvent,
	})
	return err
}

// CommitObject adds a pending object to the database. If another committed object is under target location
// it will be deleted.
func (db *DB) CommitObject(ctx context.Context, opts CommitObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	var metrics commitMetrics
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, TransactionOptions{
		MaxCommitDelay: opts.MaxCommitDelay,
		TransactionTag: "commit-object",
		TransmitEvent:  opts.TransmitEvent,
	}, func(ctx context.Context, adapter TransactionAdapter) error {
		query, err := adapter.precommitQuery(ctx, PrecommitQuery{
			ObjectStream: opts.ObjectStream,
			Pending:      true,
			ExcludeFromPending: ExcludeFromPending{
				Object:            opts.SkipPendingObject,
				ExpiresAt:         true,                           // we are getting ExpiresAt from opts
				EncryptedUserData: opts.OverrideEncryptedMetadata, // we are getting EncryptedUserData from opts
			},
			Unversioned:    !opts.Versioned,
			HighestVisible: opts.IfNoneMatch.All(),
		})
		if err != nil {
			return err
		}

		// We should only commit when an object already doesn't exist.
		if opts.IfNoneMatch.All() {
			if query.HighestVisible.IsCommitted() {
				return ErrFailedPrecondition.New("object already exists")
			}
		}

		reusePreviousObject := reusePreviousObject(opts.Versioned, query.Unversioned, query.HighestVersion)

		// When committing unversioned objects we need to delete any previous unversioned objects.
		if !opts.Versioned {
			if err := db.precommitDeleteUnversioned(ctx, adapter, query, &metrics, precommitDeleteUnversioned{
				DisallowDelete:     opts.DisallowDelete,
				BypassGovernance:   false,
				DeleteOnlySegments: reusePreviousObject,
			}); err != nil {
				return err
			}
		}

		if err = db.validateParts(query.Segments); err != nil {
			return err
		}

		var finalSegments []segmentToCommit

		if opts.SpecificSegments {
			var segmentsToDelete []SegmentPosition
			finalSegments, segmentsToDelete, err = determineCommitActions(opts.OnlySegments, query.Segments)
			if err != nil {
				return err
			}

			deletedSegmentCount, err := adapter.deleteSegmentsNotInCommit(ctx, opts.StreamID, segmentsToDelete)
			if err != nil {
				return err
			}
			metrics.DeletedSegmentCount += int(deletedSegmentCount)
		} else {
			finalSegments = convertToFinalSegments(query.Segments)
		}

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

		// Calculate what the new object should be
		{
			object.StreamID = opts.StreamID
			object.ProjectID = opts.ProjectID
			object.BucketName = opts.BucketName
			object.ObjectKey = opts.ObjectKey
			if reusePreviousObject {
				// When reusing an unversioned object, we keep the same version number
				// but update with the new StreamID. The old segments (with the old StreamID)
				// are deleted by precommitDeleteUnversioned with DeleteOnlySegments=true.
				object.Version = query.Unversioned.Version
			} else {
				object.Version = db.nextVersion(opts.Version, query.HighestVersion, query.TimestampVersion)
			}
			object.Status = committedWhereVersioned(opts.Versioned)
			object.SegmentCount = int32(len(finalSegments))
			object.TotalPlainSize = totalPlainSize
			object.TotalEncryptedSize = totalEncryptedSize
			object.FixedSegmentSize = fixedSegmentSize
			object.ExpiresAt = opts.ExpiresAt

			if query.Pending == nil {
				// values from options (no pending object)
				object.CreatedAt = time.Now()
				object.Retention = opts.Retention
				object.LegalHold = opts.LegalHold
				object.Encryption = opts.Encryption
				if opts.OverrideEncryptedMetadata {
					object.EncryptedUserData = opts.EncryptedUserData
				}
			} else {
				// values from the database (pending object exists)
				object.CreatedAt = query.Pending.CreatedAt
				object.Encryption = query.Pending.Encryption

				object.Retention.Mode = query.Pending.RetentionMode.Mode
				object.LegalHold = query.Pending.RetentionMode.LegalHold
				object.Retention.RetainUntil = query.Pending.RetainUntil.Time

				if opts.OverrideEncryptedMetadata {
					object.EncryptedUserData = opts.EncryptedUserData
				} else {
					object.EncryptedMetadata = query.Pending.EncryptedMetadata
					object.EncryptedMetadataNonce = query.Pending.EncryptedMetadataNonce
					object.EncryptedMetadataEncryptedKey = query.Pending.EncryptedMetadataEncryptedKey
					object.EncryptedETag = query.Pending.EncryptedETag
				}
			}

			// TODO: is this check actually necessary?
			if err := object.verifyObjectLockAndRetention(); err != nil {
				return Error.Wrap(err)
			}

			// TODO: should we allow to override existing encryption parameters or return error if don't match with opts?
			if object.Encryption.IsZero() {
				if opts.Encryption.IsZero() {
					return ErrInvalidRequest.New("Encryption is missing")
				}
				object.Encryption = opts.Encryption
			}
		}

		return adapter.finalizeObjectCommit(ctx, finalizeObjectCommit{
			Initial:                  opts.ObjectStream,
			Object:                   &object,
			HasPendingObject:         query.Pending != nil,
			EncryptedMetadataChanged: opts.OverrideEncryptedMetadata,
		})
	})
	if err != nil {
		return Object{}, err
	}

	metrics.submit()

	mon.Meter("object_commit").Mark(1)
	mon.IntVal("object_commit_segments").Observe(int64(object.SegmentCount))
	mon.IntVal("object_commit_encrypted_size").Observe(object.TotalEncryptedSize)

	return object, nil
}

type finalizeObjectCommit struct {
	Initial          ObjectStream
	Object           *Object
	HasPendingObject bool

	EncryptedMetadataChanged bool
}

func (ptx *postgresTransactionAdapter) finalizeObjectCommit(ctx context.Context, opts finalizeObjectCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO this implementation is not optimal for pg/crdb as we can update primary key here
	// but we made it this way to keep code simpler and consistent between spanner and pg/crdb

	initial := opts.Initial
	object := opts.Object

	// Pending object exists
	if object.Version == initial.Version && opts.HasPendingObject {
		updateColumns := []string{
			"status = $5",
			"segment_count = $6",
			"total_plain_size = $7",
			"total_encrypted_size = $8",
			"fixed_segment_size = $9",
			"zombie_deletion_deadline = NULL",
			"encryption = $10",
		}
		values := []any{
			initial.ProjectID, initial.BucketName, initial.ObjectKey, initial.Version,
			object.Status,
			object.SegmentCount,
			object.TotalPlainSize,
			object.TotalEncryptedSize,
			object.FixedSegmentSize,
			object.Encryption,
		}

		if opts.EncryptedMetadataChanged {
			updateColumns = append(updateColumns,
				"encrypted_metadata_nonce = $11",
				"encrypted_metadata = $12",
				"encrypted_metadata_encrypted_key = $13",
				"encrypted_etag = $14",
			)
			values = append(values,
				object.EncryptedMetadataNonce,
				object.EncryptedMetadata,
				object.EncryptedMetadataEncryptedKey,
				object.EncryptedETag,
			)
		}

		var result sql.Result
		result, err = ptx.tx.ExecContext(ctx, `
			UPDATE objects SET `+strings.Join(updateColumns, ", ")+`
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
		`, values...)
		if err != nil {
			return Error.New("failed to update object: %w", err)
		}
		if count, err := result.RowsAffected(); count != 1 || err != nil {
			return Error.New("failed to update object (changed %d rows): %w", count, err)
		}

		return nil
	}

	if err := postgresInsertOrUpdateObject(ctx, ptx.tx, (*RawObject)(object)); err != nil {
		return Error.New("failed to insert or update object: %w", err)
	}

	if opts.HasPendingObject {
		_, err = ptx.tx.ExecContext(ctx, `
			DELETE FROM objects
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
		`, initial.ProjectID, initial.BucketName, initial.ObjectKey, initial.Version)
		if err != nil {
			return Error.New("failed to delete pending object: %w", err)
		}
	}

	return nil
}

func (stx *spannerTransactionAdapter) finalizeObjectCommit(ctx context.Context, opts finalizeObjectCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	initial := opts.Initial
	object := opts.Object

	// Pending object exists
	if object.Version == initial.Version && opts.HasPendingObject {
		updateMap := map[string]any{
			"project_id":           initial.ProjectID,
			"bucket_name":          initial.BucketName,
			"object_key":           initial.ObjectKey,
			"version":              initial.Version,
			"status":               object.Status,
			"expires_at":           object.ExpiresAt,
			"segment_count":        int64(object.SegmentCount),
			"total_plain_size":     object.TotalPlainSize,
			"total_encrypted_size": object.TotalEncryptedSize,
			"fixed_segment_size":   int64(object.FixedSegmentSize),
			"encryption":           object.Encryption,
			"retention_mode": lockModeWrapper{
				retentionMode: &object.Retention.Mode,
				legalHold:     &object.LegalHold,
			},
			"retain_until":             timeWrapper{&object.Retention.RetainUntil},
			"zombie_deletion_deadline": nil,
		}

		if opts.EncryptedMetadataChanged {
			updateMap["encrypted_metadata_nonce"] = object.EncryptedMetadataNonce
			updateMap["encrypted_metadata"] = object.EncryptedMetadata
			updateMap["encrypted_metadata_encrypted_key"] = object.EncryptedMetadataEncryptedKey
			updateMap["encrypted_etag"] = object.EncryptedETag
		}

		err = stx.tx.BufferWrite([]*spanner.Mutation{
			spanner.UpdateMap("objects", updateMap),
		})
		if err != nil {
			return Error.New("failed to update object: %w", err)
		}

		return nil
	}

	mutations := make([]*spanner.Mutation, 0, 2)
	mutations = append(mutations, spannerInsertOrUpdateObject(RawObject(*object)))

	if opts.HasPendingObject {
		mutations = append(mutations, spanner.Delete("objects", spanner.Key{
			initial.ProjectID,
			initial.BucketName,
			initial.ObjectKey,
			int64(initial.Version),
		}))
	}

	err = stx.tx.BufferWrite(mutations)
	if err != nil {
		return Error.New("failed to update object: %w", err)
	}

	return nil
}

func (ptx *postgresTransactionAdapter) precommitDeleteExactSegments(ctx context.Context, streamID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = ptx.tx.ExecContext(ctx, `
		DELETE FROM segments
		WHERE stream_id = $1
	`, streamID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (ptx *postgresTransactionAdapter) precommitDeleteExactObject(ctx context.Context, opts ObjectStream) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = ptx.tx.ExecContext(ctx, `
		DELETE FROM objects
		WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = ptx.tx.ExecContext(ctx, `
		DELETE FROM segments
		WHERE stream_id = $1
	`, opts.StreamID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (stx *spannerTransactionAdapter) precommitDeleteExactSegments(ctx context.Context, streamID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = stx.tx.BufferWrite([]*spanner.Mutation{
		spanner.Delete("segments", spanner.KeyRange{
			Start: spanner.Key{streamID},
			End:   spanner.Key{streamID},
			Kind:  spanner.ClosedClosed,
		}),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (stx *spannerTransactionAdapter) precommitDeleteExactObject(ctx context.Context, opts ObjectStream) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = stx.tx.BufferWrite([]*spanner.Mutation{
		spanner.Delete("objects", spanner.Key{
			opts.ProjectID,
			opts.BucketName,
			opts.ObjectKey,
			opts.Version,
		}),
		spanner.Delete("segments", spanner.KeyRange{
			Start: spanner.Key{opts.StreamID},
			End:   spanner.Key{opts.StreamID},
			Kind:  spanner.ClosedClosed,
		}),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (db *DB) validateParts(segments []PrecommitSegment) error {
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

// CommitInlineObject contains arguments necessary for committing an inline object.
type CommitInlineObject struct {
	ObjectStream
	CommitInlineSegment CommitInlineSegment

	ExpiresAt *time.Time

	EncryptedUserData
	Encryption storj.EncryptionParameters

	Retention Retention // optional
	LegalHold bool

	DisallowDelete bool

	// Versioned indicates whether an object is allowed to have multiple versions.
	Versioned bool

	// IfNoneMatch is an optional field for conditional writes.
	IfNoneMatch IfNoneMatch

	// supported only by Spanner.
	TransmitEvent bool
}

// Verify verifies reqest fields.
func (c *CommitInlineObject) Verify() error {
	if err := c.ObjectStream.Verify(); err != nil {
		return err
	}

	if err := c.CommitInlineSegment.Verify(); err != nil {
		return err
	}

	if c.Encryption.CipherSuite != storj.EncUnspecified && c.Encryption.BlockSize <= 0 {
		return ErrInvalidRequest.New("Encryption.BlockSize is negative or zero")
	}

	err := c.EncryptedUserData.Verify()
	if err != nil {
		return err
	}

	if err := c.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if c.ExpiresAt != nil {
		switch {
		case c.Retention.Enabled():
			return ErrInvalidRequest.New("ExpiresAt must not be set if Retention is set")
		case c.LegalHold:
			return ErrInvalidRequest.New("ExpiresAt must not be set if LegalHold is set")
		}
	}

	return c.IfNoneMatch.Verify()
}

// CommitInlineObject adds full inline object to the database. If another committed object is under target location
// it will be deleted.
func (db *DB) CommitInlineObject(ctx context.Context, opts CommitInlineObject) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	var metrics commitMetrics
	err = db.ChooseAdapter(opts.ProjectID).WithTx(ctx, TransactionOptions{
		TransactionTag: "commit-inline-object",
		TransmitEvent:  opts.TransmitEvent,
	}, func(ctx context.Context, adapter TransactionAdapter) error {
		// TODO: verify that a pending object doesn't exist already.
		query, err := db.PrecommitQuery(ctx, PrecommitQuery{
			ObjectStream:   opts.ObjectStream,
			Pending:        false,
			Unversioned:    !opts.Versioned,
			HighestVisible: opts.IfNoneMatch.All(),
		}, adapter)
		if err != nil {
			return err
		}

		// We should only commit when an object already doesn't exist.
		if opts.IfNoneMatch.All() {
			if query.HighestVisible.IsCommitted() {
				return ErrFailedPrecondition.New("object already exists")
			}
		}

		reusePreviousObject := reusePreviousObject(opts.Versioned, query.Unversioned, query.HighestVersion)

		// When committing unversioned objects we need to delete any previous unversioned objects.
		if !opts.Versioned {
			if err := db.precommitDeleteUnversioned(ctx, adapter, query, &metrics, precommitDeleteUnversioned{
				DisallowDelete:     opts.DisallowDelete,
				BypassGovernance:   false,
				DeleteOnlySegments: reusePreviousObject,
			}); err != nil {
				return err
			}
		}

		now := time.Now() // TODO: should we get this information from the database?

		{
			object.StreamID = opts.StreamID
			object.ProjectID = opts.ProjectID
			object.BucketName = opts.BucketName
			object.ObjectKey = opts.ObjectKey
			object.CreatedAt = now
			if reusePreviousObject {
				// When reusing an unversioned object, we keep the same version number
				// but update with the new StreamID. The old segments (with the old StreamID)
				// are deleted by precommitDeleteUnversioned with DeleteOnlySegments=true.
				object.Version = query.Unversioned.Version
			} else {
				object.Version = db.nextVersion(opts.Version, query.HighestVersion, query.TimestampVersion)
			}
			object.Status = committedWhereVersioned(opts.Versioned)
			object.SegmentCount = 1
			object.TotalPlainSize = int64(opts.CommitInlineSegment.PlainSize)
			object.TotalEncryptedSize = int64(int32(len(opts.CommitInlineSegment.InlineData)))
			object.ExpiresAt = opts.ExpiresAt
			object.Encryption = opts.Encryption
			object.EncryptedUserData = opts.EncryptedUserData
			object.Retention = opts.Retention
			object.LegalHold = opts.LegalHold

			// TODO: is this check actually necessary?
			if err := object.verifyObjectLockAndRetention(); err != nil {
				return Error.Wrap(err)
			}

			// TODO: should we allow to override existing encryption parameters or return error if don't match with opts?
			if object.Encryption.IsZero() {
				if opts.Encryption.IsZero() {
					return ErrInvalidRequest.New("Encryption is missing")
				}
				object.Encryption = opts.Encryption
			}
		}

		return adapter.precommitInsertOrUpdateObject(ctx, &object, []*Segment{{
			StreamID:          opts.StreamID,
			Position:          opts.CommitInlineSegment.Position,
			CreatedAt:         now,
			ExpiresAt:         opts.ExpiresAt,
			EncryptedKey:      opts.CommitInlineSegment.EncryptedKey,
			EncryptedKeyNonce: opts.CommitInlineSegment.EncryptedKeyNonce,
			EncryptedETag:     opts.CommitInlineSegment.EncryptedETag,
			PlainSize:         opts.CommitInlineSegment.PlainSize,
			EncryptedSize:     int32(len(opts.CommitInlineSegment.InlineData)),
			InlineData:        opts.CommitInlineSegment.InlineData,
		}})
	})
	if err != nil {
		return Object{}, err
	}

	metrics.submit()

	mon.Meter("object_commit").Mark(1)
	mon.IntVal("object_commit_segments").Observe(int64(object.SegmentCount))
	mon.IntVal("object_commit_encrypted_size").Observe(object.TotalEncryptedSize)

	return object, nil
}

type precommitDeleteUnversioned struct {
	DisallowDelete     bool
	BypassGovernance   bool
	DeleteOnlySegments bool
}

func (db *DB) precommitDeleteUnversioned(ctx context.Context, adapter TransactionAdapter, query *PrecommitInfo, metrics *commitMetrics, opts precommitDeleteUnversioned) (err error) {
	if query.Unversioned == nil {
		return nil
	}

	// If we are not allowed to delete the object we cannot commit.
	if opts.DisallowDelete {
		return ErrPermissionDenied.New("no permissions to delete existing object")
	}

	// Retention is not allowed for unversioned objects,
	// however the check is cheap and we rather not lose protected objects.
	var retention Retention
	retention.Mode = query.Unversioned.RetentionMode.Mode
	retention.RetainUntil = query.Unversioned.RetainUntil.Time

	// If the object has a legal hold and retention, we also cannot commit.
	if err = retention.Verify(); err != nil {
		return Error.Wrap(err)
	}
	switch {
	case query.Unversioned.RetentionMode.LegalHold:
		return ErrObjectLock.New(legalHoldErrMsg)
	case retention.isProtected(opts.BypassGovernance, time.Now()):
		return ErrObjectLock.New(retentionErrMsg)
	}

	if opts.DeleteOnlySegments {
		if err = adapter.precommitDeleteExactSegments(ctx, query.Unversioned.StreamID); err != nil {
			return Error.Wrap(err)
		}
	} else {
		// delete the previous unversioned object
		if err := adapter.precommitDeleteExactObject(ctx, ObjectStream{
			ProjectID:  query.ProjectID,
			BucketName: query.BucketName,
			ObjectKey:  query.ObjectKey,
			Version:    query.Unversioned.Version,
			StreamID:   query.Unversioned.StreamID,
		}); err != nil {
			return Error.Wrap(err)
		}
		// update the metrics
		metrics.DeletedObjectCount = 1
	}

	return nil
}

func (ptx *postgresTransactionAdapter) precommitInsertSegments(ctx context.Context, segments []*Segment) (err error) {
	t, err := transposeSegments(segments, func(p Pieces) ([]byte, error) {
		if len(p) != 0 {
			return nil, Error.New("expected only inline segments")
		}
		return nil, nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = ptx.tx.ExecContext(ctx, `
			INSERT INTO segments (
				stream_id, position, expires_at,
				encrypted_key_nonce, encrypted_key,
				root_piece_id,
				redundancy,
				encrypted_size, plain_offset, plain_size,
				remote_alias_pieces, placement,
				inline_data
			) SELECT
				$1, UNNEST($2::INT8[]), UNNEST($3::timestamptz[]),
				UNNEST($4::BYTEA[]), UNNEST($5::BYTEA[]),
				UNNEST($6::BYTEA[]),
				UNNEST($7::INT8[]),
				UNNEST($8::INT4[]), UNNEST($9::INT8[]),	UNNEST($10::INT4[]),
				UNNEST($11::BYTEA[]), UNNEST($12::INT2[]),
				UNNEST($13::BYTEA[])
		`, t.StreamID, pgutil.Int8Array(t.Positions), pgutil.NullTimestampTZArray(t.ExpiresAts),
		pgutil.ByteaArray(t.EncryptedKeyNonces), pgutil.ByteaArray(t.EncryptedKeys),
		pgutil.ByteaArray(t.RootPieceIDs),
		pgutil.Int8Array(t.RedundancySchemes),
		pgutil.Int4Array(t.EncryptedSizes), pgutil.Int8Array(t.PlainOffsets), pgutil.Int4Array(t.PlainSizes),
		pgutil.ByteaArray(t.PiecesLists), pgutil.PlacementConstraintArray(t.Placements),
		pgutil.ByteaArray(t.InlineDatas),
	)
	if err != nil {
		return Error.New("unable to insert segments: %w", err)
	}

	return nil
}

func (ptx *postgresTransactionAdapter) precommitInsertObject(ctx context.Context, object *Object, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := ptx.precommitInsertSegments(ctx, segments); err != nil {
		return err
	}

	if err := postgresInsertObject(ctx, ptx.tx, (*RawObject)(object)); err != nil {
		return Error.New("unable to insert object: %w", err)
	}

	return nil
}

func (ptx *postgresTransactionAdapter) precommitInsertOrUpdateObject(ctx context.Context, object *Object, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := ptx.precommitInsertSegments(ctx, segments); err != nil {
		return err
	}

	if err := postgresInsertOrUpdateObject(ctx, ptx.tx, (*RawObject)(object)); err != nil {
		return Error.New("unable to insert or update object: %w", err)
	}

	return nil
}

func (stx *spannerTransactionAdapter) precommitInsertObject(ctx context.Context, object *Object, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	var mutations []*spanner.Mutation

	mutations = append(mutations, spannerInsertObject(RawObject(*object)))
	for _, segment := range segments {
		if len(segment.Pieces) != 0 {
			return Error.New("internal error: tried to insert segment with remote alias pieces")
		}
		mutations = append(mutations, spannerInsertSegment(RawSegment(*segment), nil))
	}

	err = stx.tx.BufferWrite(mutations)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (stx *spannerTransactionAdapter) precommitInsertOrUpdateObject(ctx context.Context, object *Object, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	var mutations []*spanner.Mutation

	mutations = append(mutations, spannerInsertOrUpdateObject(RawObject(*object)))
	for _, segment := range segments {
		if len(segment.Pieces) != 0 {
			return Error.New("internal error: tried to insert segment with remote alias pieces")
		}
		mutations = append(mutations, spannerInsertSegment(RawSegment(*segment), nil))
	}

	err = stx.tx.BufferWrite(mutations)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// reusePreviousObject determines whether to reuse the previous unversioned object
// as a new committed object. This indicates that we can do single INSERT OR UPDATE
// instead of INSERT + DELETE when finalizing the object commit.
func reusePreviousObject(newVersioned bool, unversioned *PrecommitUnversionedObject, highestVisible Version) bool {
	return !newVersioned && unversioned != nil && unversioned.Version == highestVisible
}
