// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
)

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
