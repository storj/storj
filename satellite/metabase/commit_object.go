// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
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

type commitObjectWithSegmentsTransactionAdapter interface {
	fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []PrecommitSegment, err error)
	deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error)

	precommitTransactionAdapter
}

func verifySegmentOrder(positions []SegmentPosition) error {
	if len(positions) == 0 {
		return nil
	}

	last := positions[0]
	for _, next := range positions[1:] {
		if !last.Less(next) {
			return Error.New("segments not in ascending order, got %v before %v", last, next)
		}
		last = next
	}

	return nil
}

// PrecommitSegment is segment state before committing the object.
type PrecommitSegment struct {
	Position      SegmentPosition
	EncryptedSize int32
	PlainOffset   int64
	PlainSize     int32
}

// fetchSegmentsForCommit loads information necessary for validating segment existence and offsets.
func (ptx *postgresTransactionAdapter) fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []PrecommitSegment, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(ptx.tx.QueryContext(ctx, `
		SELECT position, encrypted_size, plain_offset, plain_size
		FROM segments
		WHERE stream_id = $1
		ORDER BY position
	`, streamID))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment PrecommitSegment
			err := rows.Scan(&segment.Position, &segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}
			segments = append(segments, segment)
		}
		return nil
	})
	if err != nil {
		return nil, Error.New("failed to fetch segments: %w", err)
	}
	return segments, nil
}

func (stx *spannerTransactionAdapter) fetchSegmentsForCommit(ctx context.Context, streamID uuid.UUID) (segments []PrecommitSegment, err error) {
	defer mon.Task()(&ctx)(&err)

	const maxPosition = int64(math.MaxInt64)
	keyRange := spanner.KeyRange{
		// Key: StreamID, Position
		Start: spanner.Key{streamID.Bytes()},
		End:   spanner.Key{streamID.Bytes(), maxPosition},
		Kind:  spanner.ClosedClosed, // both keys are included.
	}

	segments, err = spannerutil.CollectRows(stx.tx.ReadWithOptions(ctx, "segments", keyRange,
		[]string{"position", "encrypted_size", "plain_offset", "plain_size"},
		&spanner.ReadOptions{RequestTag: "fetch-segments-for-commit"},
	), func(row *spanner.Row, segment *PrecommitSegment) error {
		return Error.Wrap(row.Columns(
			&segment.Position, spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
		))
	})

	return segments, Error.Wrap(err)
}

type segmentToCommit struct {
	Position       SegmentPosition
	OldPlainOffset int64
	PlainSize      int32
	EncryptedSize  int32
}

// determineCommitActions detects how should the database be updated and which segments should be deleted.
func determineCommitActions(segments []SegmentPosition, segmentsInDatabase []PrecommitSegment) (commit []segmentToCommit, toDelete []SegmentPosition, err error) {
	var invalidSegments errs.Group

	commit = make([]segmentToCommit, 0, len(segments))
	diffSegmentsWithDatabase(segments, segmentsInDatabase, func(a *SegmentPosition, b *PrecommitSegment) {
		// If we do not have an appropriate segment in the database it means
		// either the segment was deleted before commit finished or the
		// segment was not uploaded. Either way we need to fail the commit.
		if b == nil {
			invalidSegments.Add(fmt.Errorf("%v: segment not committed", *a))
			return
		}

		// If we do not commit a segment that's in a database we should delete them.
		// This could happen when the user tries to upload a segment,
		// fails, reuploads and then during commit decides to not commit into the object.
		if a == nil {
			toDelete = append(toDelete, b.Position)
			return
		}

		commit = append(commit, segmentToCommit{
			Position:       *a,
			OldPlainOffset: b.PlainOffset,
			PlainSize:      b.PlainSize,
			EncryptedSize:  b.EncryptedSize,
		})
	})

	if err := invalidSegments.Err(); err != nil {
		return nil, nil, Error.New("segments and database does not match: %v", err)
	}
	return commit, toDelete, nil
}

// convertToFinalSegments converts PrecommitSegment to segmentToCommit.
func convertToFinalSegments(segmentsInDatabase []PrecommitSegment) (commit []segmentToCommit) {
	commit = make([]segmentToCommit, 0, len(segmentsInDatabase))
	for _, seg := range segmentsInDatabase {
		commit = append(commit, segmentToCommit{
			Position:       seg.Position,
			OldPlainOffset: seg.PlainOffset,
			PlainSize:      seg.PlainSize,
			EncryptedSize:  seg.EncryptedSize,
		})
	}
	return commit
}

// updateSegmentOffsets updates segment offsets that didn't match the database state.
func (ptx *postgresTransactionAdapter) updateSegmentOffsets(ctx context.Context, streamID uuid.UUID, updates []segmentToCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	// When none of the segments have changed, then the update will be skipped.

	// Update plain offsets of the segments.
	var batch struct {
		Positions    []int64
		PlainOffsets []int64
	}
	expectedOffset := int64(0)
	for _, u := range updates {
		if u.OldPlainOffset != expectedOffset {
			batch.Positions = append(batch.Positions, int64(u.Position.Encode()))
			batch.PlainOffsets = append(batch.PlainOffsets, expectedOffset)
		}
		expectedOffset += int64(u.PlainSize)
	}
	if len(batch.Positions) == 0 {
		return nil
	}

	updateResult, err := ptx.tx.ExecContext(ctx, `
		UPDATE segments
		SET plain_offset = P.plain_offset
		FROM (SELECT unnest($2::INT8[]), unnest($3::INT8[])) as P(position, plain_offset)
		WHERE segments.stream_id = $1 AND segments.position = P.position
	`, streamID, pgutil.Int8Array(batch.Positions), pgutil.Int8Array(batch.PlainOffsets))
	if err != nil {
		return Error.New("unable to update segments offsets: %w", err)
	}

	affected, err := updateResult.RowsAffected()
	if err != nil {
		return Error.New("unable to get number of affected segments: %w", err)
	}
	if affected != int64(len(batch.Positions)) {
		return Error.New("not all segments were updated, expected %d got %d", len(batch.Positions), affected)
	}

	return nil
}

func (stx *spannerTransactionAdapter) updateSegmentOffsets(ctx context.Context, streamID uuid.UUID, updates []segmentToCommit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(updates) == 0 {
		return nil
	}

	// When none of the segments have changed, then the update will be skipped.

	// Update plain offsets of the segments.
	var mutations []*spanner.Mutation
	expectedOffset := int64(0)
	for _, u := range updates {
		if u.OldPlainOffset != expectedOffset {
			mutations = append(mutations, spanner.Update("segments",
				[]string{"stream_id", "position", "plain_offset"},
				[]interface{}{streamID, u.Position, expectedOffset}),
			)
		}
		expectedOffset += int64(u.PlainSize)
	}
	if len(mutations) == 0 {
		return nil
	}

	err = stx.tx.BufferWrite(mutations)
	if err != nil {
		return Error.New("unable to update segments offsets: %w", err)
	}
	return nil
}

// deleteSegmentsNotInCommit deletes the listed segments inside the tx.
func (ptx *postgresTransactionAdapter) deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(segments) == 0 {
		return 0, nil
	}

	positions := []int64{}
	for _, p := range segments {
		positions = append(positions, int64(p.Encode()))
	}

	// This potentially could be done together with the previous database call.
	result, err := ptx.tx.ExecContext(ctx, `
		DELETE FROM segments
		WHERE stream_id = $1 AND position = ANY($2)
	`, streamID, pgutil.Int8Array(positions))
	if err != nil {
		return 0, Error.New("unable to delete segments: %w", err)
	}

	deletedCount, err := result.RowsAffected()
	if err != nil {
		return 0, Error.New("unable to count deleted segments: %w", err)
	}

	return deletedCount, nil
}

func (stx *spannerTransactionAdapter) deleteSegmentsNotInCommit(ctx context.Context, streamID uuid.UUID, segments []SegmentPosition) (deletedSegmentCount int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(segments) == 0 {
		return 0, nil
	}

	var mutations []*spanner.Mutation
	for _, pos := range segments {
		mutations = append(mutations,
			spanner.Delete("segments", spanner.Key{streamID, int64(pos.Encode())}))
	}

	err = stx.tx.BufferWrite(mutations)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return int64(len(segments)), nil
}

// diffSegmentsWithDatabase matches up segment positions with their database information.
func diffSegmentsWithDatabase(as []SegmentPosition, bs []PrecommitSegment, cb func(a *SegmentPosition, b *PrecommitSegment)) {
	for len(as) > 0 && len(bs) > 0 {
		if as[0] == bs[0].Position {
			cb(&as[0], &bs[0])
			as, bs = as[1:], bs[1:]
		} else if as[0].Less(bs[0].Position) {
			cb(&as[0], nil)
			as = as[1:]
		} else {
			cb(nil, &bs[0])
			bs = bs[1:]
		}
	}
	for i := range as {
		cb(&as[i], nil)
	}
	for i := range bs {
		cb(nil, &bs[i])
	}
}

// reusePreviousObject determines whether to reuse the previous unversioned object
// as a new committed object. This indicates that we can do single INSERT OR UPDATE
// instead of INSERT + DELETE when finalizing the object commit.
func reusePreviousObject(newVersioned bool, unversioned *PrecommitUnversionedObject, highestVisible Version) bool {
	return !newVersioned && unversioned != nil && unversioned.Version == highestVisible
}
