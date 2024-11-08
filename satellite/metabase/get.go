// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// ErrSegmentNotFound is an error class for non-existing segment.
var ErrSegmentNotFound = errs.Class("segment not found")

// Object object metadata.
// TODO define separated struct.
type Object RawObject

// IsMigrated returns whether the object comes from PointerDB.
// Pointer objects are special that they are missing some information.
//
//   - TotalPlainSize = 0 and FixedSegmentSize = 0.
//   - Segment.PlainOffset = 0, Segment.PlainSize = 0
func (obj *Object) IsMigrated() bool {
	return obj.TotalPlainSize <= 0
}

// StreamVersionID returns byte representation of object stream version id.
func (obj *Object) StreamVersionID() StreamVersionID {
	return NewStreamVersionID(obj.Version, obj.StreamID)
}

// Segment segment metadata.
// TODO define separated struct.
type Segment RawSegment

// Inline returns true if segment is inline.
func (s Segment) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// Expired checks if segment is expired relative to now.
func (s Segment) Expired(now time.Time) bool {
	return s.ExpiresAt != nil && s.ExpiresAt.Before(now)
}

// PieceSize returns calculated piece size for segment.
func (s Segment) PieceSize() int64 {
	return s.Redundancy.PieceSize(int64(s.EncryptedSize))
}

// GetObjectExactVersion contains arguments necessary for fetching an information
// about exact object version.
type GetObjectExactVersion struct {
	Version Version
	ObjectLocation
}

// Verify verifies get object request fields.
func (obj *GetObjectExactVersion) Verify() error {
	if err := obj.ObjectLocation.Verify(); err != nil {
		return err
	}
	if obj.Version <= 0 {
		return ErrInvalidRequest.New("Version invalid: %v", obj.Version)
	}
	return nil
}

// GetObjectExactVersion returns object information for exact version.
func (db *DB) GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	object, err := db.ChooseAdapter(opts.ProjectID).GetObjectExactVersion(ctx, opts)
	if err != nil {
		return Object{}, err
	}
	return object, nil
}

// GetObjectExactVersion returns object information for exact version.
func (p *PostgresAdapter) GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (_ Object, err error) {
	object := Object{}
	err = p.db.QueryRowContext(ctx, `
		SELECT
			stream_id, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			retention_mode, retain_until
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4) AND
			status <> `+statusPending+` AND
			(expires_at IS NULL OR expires_at > now())`,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version).
		Scan(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			&object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
			lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
			timeWrapper{&object.Retention.RetainUntil},
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Object{}, Error.New("unable to query object status: %w", err)
	}

	if err = object.Retention.Verify(); err != nil {
		return Object{}, Error.Wrap(err)
	}

	object.ProjectID = opts.ProjectID
	object.BucketName = opts.BucketName
	object.ObjectKey = opts.ObjectKey
	object.Version = opts.Version

	return object, nil
}

// GetObjectExactVersion returns object information for exact version.
func (s *SpannerAdapter) GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (object Object, err error) {
	object, err = spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				stream_id, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				retention_mode, retain_until
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version) AND
				status <> ` + statusPending + ` AND
				(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
		},
	}), func(row *spanner.Row, object *Object) error {
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		object.Version = opts.Version

		return Error.Wrap(row.Columns(
			&object.StreamID, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			spannerutil.Int(&object.SegmentCount),
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
			encryptionParameters{&object.Encryption},
			lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
			timeWrapper{&object.Retention.RetainUntil},
		))
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return Object{}, Error.New("unable to query object status: %w", err)
	}

	if err = object.Retention.Verify(); err != nil {
		return Object{}, Error.Wrap(err)
	}

	return object, nil
}

// GetObjectLastCommitted contains arguments necessary for fetching
// an object information for last committed version.
type GetObjectLastCommitted struct {
	ObjectLocation
}

// GetObjectLastCommitted returns object information for last committed version.
func (db *DB) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted) (_ Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	return db.ChooseAdapter(opts.ProjectID).GetObjectLastCommitted(ctx, opts)
}

// GetObjectLastCommitted implements Adapter.
func (p *PostgresAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted) (object Object, err error) {
	object.ProjectID = opts.ProjectID
	object.BucketName = opts.BucketName
	object.ObjectKey = opts.ObjectKey

	err = p.db.QueryRowContext(ctx, `
		SELECT
			stream_id, version, status,
			created_at, expires_at,
			segment_count,
			clear_metadata,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			retention_mode, retain_until
		FROM objects
		WHERE
			(project_id, bucket_name, object_key) = ($1, $2, $3) AND
			status <> `+statusPending+` AND
			(expires_at IS NULL OR expires_at > now())
		ORDER BY version DESC
		LIMIT 1`,
		opts.ProjectID, opts.BucketName, opts.ObjectKey,
	).Scan(
		&object.StreamID, &object.Version, &object.Status,
		&object.CreatedAt, &object.ExpiresAt,
		&object.SegmentCount,
		&object.ClearMetadata,
		&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
		&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
		encryptionParameters{&object.Encryption},
		lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
		timeWrapper{&object.Retention.RetainUntil},
	)

	if errors.Is(err, sql.ErrNoRows) || object.Status.IsDeleteMarker() {
		return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
	}
	if err != nil {
		return Object{}, Error.Wrap(err)
	}

	if err = object.Retention.Verify(); err != nil {
		return Object{}, Error.Wrap(err)
	}

	return object, nil
}

// GetObjectLastCommitted implements Adapter.
func (s *SpannerAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted) (object Object, err error) {
	object, err = spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				stream_id, version, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				retention_mode, retain_until
			FROM objects
			WHERE
				project_id = @project_id AND
				bucket_name = @bucket_name AND
				object_key = @object_key AND
				status <> ` + statusPending + ` AND
				(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
			ORDER BY version DESC
			LIMIT 1`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}), func(row *spanner.Row, object *Object) error {
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey

		return Error.Wrap(row.Columns(
			&object.StreamID, &object.Version, &object.Status,
			&object.CreatedAt, &object.ExpiresAt,
			spannerutil.Int(&object.SegmentCount),
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
			encryptionParameters{&object.Encryption},
			lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
			timeWrapper{&object.Retention.RetainUntil},
		))
	})
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return Object{}, Error.Wrap(err)
	}
	if object.Status.IsDeleteMarker() {
		return Object{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
	}

	if err = object.Retention.Verify(); err != nil {
		return Object{}, Error.Wrap(err)
	}

	return object, nil
}

// GetSegmentByPosition contains arguments necessary for fetching a segment on specific position.
type GetSegmentByPosition struct {
	StreamID uuid.UUID
	Position SegmentPosition
}

// Verify verifies get segment request fields.
func (seg *GetSegmentByPosition) Verify() error {
	if seg.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}
	return nil
}

// GetSegmentByPosition returns information about segment on the specified position.
func (db *DB) GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (segment Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Segment{}, err
	}

	// check all adapters until a match is found
	var aliasPieces AliasPieces
	found := false
	for _, adapter := range db.adapters {
		segment, aliasPieces, err = adapter.GetSegmentByPosition(ctx, opts)
		if err != nil {
			if ErrSegmentNotFound.Has(err) {
				continue
			}
			return Segment{}, err
		}
		found = true
		break
	}
	if !found {
		return Segment{}, ErrSegmentNotFound.New("segment missing")
	}

	if len(aliasPieces) > 0 {
		segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			return Segment{}, Error.New("unable to convert aliases to pieces: %w", err)
		}
	}

	segment.StreamID = opts.StreamID
	segment.Position = opts.Position

	return segment, nil
}

// GetSegmentByPosition returns information about segment on the specified position.
func (p *PostgresAdapter) GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (segment Segment, aliasPieces AliasPieces, err error) {
	err = p.db.QueryRowContext(ctx, `
		SELECT
			created_at, expires_at, repaired_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			encrypted_etag,
			redundancy,
			inline_data, remote_alias_pieces,
			placement
		FROM segments
		WHERE (stream_id, position) = ($1, $2)
	`, opts.StreamID, opts.Position.Encode()).
		Scan(
			&segment.CreatedAt, &segment.ExpiresAt, &segment.RepairedAt,
			&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			&segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize,
			&segment.EncryptedETag,
			redundancyScheme{&segment.Redundancy},
			&segment.InlineData, &aliasPieces,
			&segment.Placement,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Segment{}, nil, ErrSegmentNotFound.New("segment missing")
		}
		return Segment{}, nil, Error.New("unable to query segment: %w", err)
	}

	return segment, aliasPieces, err
}

// GetSegmentByPosition returns information about segment on the specified position.
func (s *SpannerAdapter) GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (segment Segment, aliasPieces AliasPieces, err error) {
	segment, err = spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				created_at, expires_at, repaired_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size,
				encrypted_etag,
				redundancy,
				inline_data, remote_alias_pieces,
				placement
			FROM segments
			WHERE (stream_id, position) = (@stream_id, @position)
		`,
		Params: map[string]interface{}{
			"stream_id": opts.StreamID,
			"position":  opts.Position,
		},
	}), func(row *spanner.Row, segment *Segment) error {
		return Error.Wrap(row.Columns(
			&segment.CreatedAt, &segment.ExpiresAt, &segment.RepairedAt,
			&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
			&segment.EncryptedETag,
			redundancyScheme{&segment.Redundancy},
			&segment.InlineData, &aliasPieces,
			&segment.Placement,
		))
	})
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Segment{}, nil, ErrSegmentNotFound.New("segment missing")
		}
		return Segment{}, nil, Error.New("unable to query segment: %w", err)
	}

	return segment, aliasPieces, nil
}

// GetLatestObjectLastSegment contains arguments necessary for fetching a last segment information.
type GetLatestObjectLastSegment struct {
	ObjectLocation
}

// GetLatestObjectLastSegment returns an object last segment information.
func (db *DB) GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (segment Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Segment{}, err
	}

	segment, aliasPieces, err := db.ChooseAdapter(opts.ProjectID).GetLatestObjectLastSegment(ctx, opts)
	if err != nil {
		return Segment{}, err
	}

	if len(aliasPieces) > 0 {
		segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			return Segment{}, Error.New("unable to convert aliases to pieces: %w", err)
		}
	}

	return segment, nil
}

// GetLatestObjectLastSegment returns an object last segment information.
func (p *PostgresAdapter) GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (segment Segment, aliasPieces AliasPieces, err error) {
	err = p.db.QueryRowContext(ctx, `
		SELECT
			stream_id, position,
			created_at, repaired_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			encrypted_etag,
			redundancy,
			inline_data, remote_alias_pieces,
			placement
		FROM segments
		WHERE
			stream_id IN (
				SELECT stream_id
				FROM objects
				WHERE
					(project_id, bucket_name, object_key) = ($1, $2, $3) AND
					status <> `+statusPending+`
					ORDER BY version DESC
					LIMIT 1
			)
		ORDER BY position DESC
		LIMIT 1
	`, opts.ProjectID, opts.BucketName, opts.ObjectKey).
		Scan(
			&segment.StreamID, &segment.Position,
			&segment.CreatedAt, &segment.RepairedAt,
			&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			&segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize,
			&segment.EncryptedETag,
			redundancyScheme{&segment.Redundancy},
			&segment.InlineData, &aliasPieces,
			&segment.Placement,
		)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Segment{}, nil, ErrObjectNotFound.Wrap(Error.New("object or segment missing"))
		}
		return Segment{}, nil, Error.New("unable to query segment: %w", err)
	}
	return segment, aliasPieces, nil
}

// GetLatestObjectLastSegment returns an object last segment information.
func (s *SpannerAdapter) GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (segment Segment, aliasPieces AliasPieces, err error) {
	segment, err = spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				stream_id, position,
				created_at, repaired_at,
				root_piece_id, encrypted_key_nonce, encrypted_key,
				encrypted_size, plain_offset, plain_size,
				encrypted_etag,
				redundancy,
				inline_data, remote_alias_pieces,
				placement
			FROM segments
			WHERE
				stream_id IN (
					SELECT stream_id
					FROM objects
					WHERE
						(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key) AND
						status <> ` + statusPending + `
						ORDER BY version DESC
						LIMIT 1
				)
			ORDER BY position DESC
			LIMIT 1
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}), func(row *spanner.Row, segment *Segment) error {
		return Error.Wrap(row.Columns(
			&segment.StreamID, &segment.Position,
			&segment.CreatedAt, &segment.RepairedAt,
			&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
			&segment.EncryptedETag,
			redundancyScheme{&segment.Redundancy},
			&segment.InlineData, &aliasPieces,
			&segment.Placement,
		))
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Segment{}, nil, ErrObjectNotFound.Wrap(Error.New("object or segment missing"))
		}
		return Segment{}, nil, Error.New("unable to read segment from query: %w", err)
	}

	return segment, aliasPieces, nil
}

// BucketEmpty contains arguments necessary for checking if bucket is empty.
type BucketEmpty struct {
	ProjectID  uuid.UUID
	BucketName BucketName
}

// BucketEmpty returns true if bucket does not contain objects (pending or committed).
// This method doesn't check bucket existence.
func (db *DB) BucketEmpty(ctx context.Context, opts BucketEmpty) (empty bool, err error) {
	defer mon.Task()(&ctx)(&err)

	switch {
	case opts.ProjectID.IsZero():
		return false, ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return false, ErrInvalidRequest.New("BucketName missing")
	}

	return db.ChooseAdapter(opts.ProjectID).BucketEmpty(ctx, opts)
}

// BucketEmpty returns true if bucket does not contain objects (pending or committed).
// This method doesn't check bucket existence.
func (p *PostgresAdapter) BucketEmpty(ctx context.Context, opts BucketEmpty) (empty bool, err error) {
	var value bool
	err = p.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM objects WHERE (project_id, bucket_name) = ($1, $2))
	`, opts.ProjectID, opts.BucketName).Scan(&value)
	if err != nil {
		return false, Error.New("unable to query objects: %w", err)
	}

	return !value, nil
}

// BucketEmpty returns true if bucket does not contain objects (pending or committed).
// This method doesn't check bucket existence.
func (s *SpannerAdapter) BucketEmpty(ctx context.Context, opts BucketEmpty) (empty bool, err error) {
	return spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `SELECT NOT EXISTS (
			SELECT 1 FROM objects WHERE (project_id, bucket_name) = (@project_id, @bucket_name)
		)`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
		},
	}), func(row *spanner.Row, noitems *bool) error {
		return Error.Wrap(row.Columns(noitems))
	})
}

// GetObjectExactVersionLegalHold contains arguments necessary for retrieving
// the legal hold configuration of an exact version of an object.
type GetObjectExactVersionLegalHold struct {
	ObjectLocation
	Version Version
}

// GetObjectExactVersionLegalHold returns the legal hold configuration of an exact version of an object.
func (db *DB) GetObjectExactVersionLegalHold(ctx context.Context, opts GetObjectExactVersionLegalHold) (enabled bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return false, err
	}

	return db.ChooseAdapter(opts.ProjectID).GetObjectExactVersionLegalHold(ctx, opts)
}

// GetObjectExactVersionLegalHold returns the legal hold configuration of an exact version of an object.
func (p *PostgresAdapter) GetObjectExactVersionLegalHold(ctx context.Context, opts GetObjectExactVersionLegalHold) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var info lockInfoAndStatus

	err = p.db.QueryRowContext(ctx, `
		SELECT retention_mode, status
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)`,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
	).Scan(lockModeWrapper{legalHold: &info.LegalHold}, &info.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return false, Error.New("unable to query object legal hold configuration: %w", err)
	}

	switch {
	case info.Status.IsDeleteMarker():
		return false, ErrMethodNotAllowed.New("querying legal hold status of delete marker is not allowed")
	case !info.Status.IsCommitted():
		return false, ErrMethodNotAllowed.New(noLockFromUncommittedErrMsg)
	}

	return info.LegalHold, nil
}

// GetObjectExactVersionLegalHold returns the legal hold configuration of an exact version of an object.
func (s *SpannerAdapter) GetObjectExactVersionLegalHold(ctx context.Context, opts GetObjectExactVersionLegalHold) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT retention_mode, status
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
		},
	}), func(row *spanner.Row, info *lockInfoAndStatus) error {
		err := row.Columns(lockModeWrapper{legalHold: &info.LegalHold}, &info.Status)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return false, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return false, Error.New("unable to query object legal hold configuration: %w", err)
	}

	switch {
	case info.Status.IsDeleteMarker():
		return false, ErrMethodNotAllowed.New("querying legal hold status of delete marker is not allowed")
	case !info.Status.IsCommitted():
		return false, ErrMethodNotAllowed.New(noLockFromUncommittedErrMsg)
	}

	return info.LegalHold, nil
}

// GetObjectLastCommittedLegalHold contains arguments necessary for retrieving the legal hold
// configuration of the most recently committed version of an object.
type GetObjectLastCommittedLegalHold struct {
	ObjectLocation
}

// GetObjectLastCommittedLegalHold returns the legal hold configuration of the most recently
// committed version of an object.
func (db *DB) GetObjectLastCommittedLegalHold(ctx context.Context, opts GetObjectLastCommittedLegalHold) (enabled bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = opts.Verify(); err != nil {
		return false, err
	}

	return db.ChooseAdapter(opts.ProjectID).GetObjectLastCommittedLegalHold(ctx, opts)
}

// GetObjectLastCommittedLegalHold returns the legal hold configuration of the most recently
// committed version of an object.
func (p *PostgresAdapter) GetObjectLastCommittedLegalHold(ctx context.Context, opts GetObjectLastCommittedLegalHold) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	var info lockInfoAndStatus

	err = p.db.QueryRowContext(ctx, `
		SELECT retention_mode, status
		FROM objects
		WHERE
			(project_id, bucket_name, object_key) = ($1, $2, $3)
			AND status <> `+statusPending+`
		ORDER BY version DESC
		LIMIT 1
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey,
	).Scan(lockModeWrapper{legalHold: &info.LegalHold}, &info.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return false, Error.New("unable to query object legal hold configuration: %w", err)
	}

	if info.Status.IsDeleteMarker() {
		return false, ErrMethodNotAllowed.New("querying legal hold status of delete marker is not allowed")
	}

	return info.LegalHold, nil
}

// GetObjectLastCommittedLegalHold returns the legal hold configuration of the most recently
// committed version of an object.
func (s *SpannerAdapter) GetObjectLastCommittedLegalHold(ctx context.Context, opts GetObjectLastCommittedLegalHold) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT retention_mode, status
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
				AND status <> ` + statusPending + `
			ORDER BY version DESC
			LIMIT 1
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}), func(row *spanner.Row, info *lockInfoAndStatus) error {
		err := row.Columns(lockModeWrapper{legalHold: &info.LegalHold}, &info.Status)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return false, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return false, Error.New("unable to query object legal hold configuration: %w", err)
	}

	if info.Status.IsDeleteMarker() {
		return false, ErrMethodNotAllowed.New("querying legal hold status of delete marker is not allowed")
	}

	return info.LegalHold, nil
}

// GetObjectExactVersionRetention contains arguments necessary for retrieving
// the retention configuration of an exact version of an object.
type GetObjectExactVersionRetention struct {
	ObjectLocation
	Version Version
}

// GetObjectExactVersionRetention returns the retention configuration of an exact version of an object.
func (db *DB) GetObjectExactVersionRetention(ctx context.Context, opts GetObjectExactVersionRetention) (retention Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Retention{}, err
	}

	retention, err = db.ChooseAdapter(opts.ProjectID).GetObjectExactVersionRetention(ctx, opts)
	if err != nil {
		return Retention{}, err
	}

	return retention, nil
}

// GetObjectExactVersionRetention returns the retention configuration of an exact version of an object.
func (p *PostgresAdapter) GetObjectExactVersionRetention(ctx context.Context, opts GetObjectExactVersionRetention) (_ Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	var info lockInfoAndStatus

	err = p.db.QueryRowContext(ctx, `
		SELECT retention_mode, retain_until, status
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)`,
		opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version,
	).Scan(lockModeWrapper{retentionMode: &info.Retention.Mode}, timeWrapper{&info.Retention.RetainUntil}, &info.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Retention{}, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Retention{}, Error.New("unable to query object retention configuration: %w", err)
	}

	switch {
	case info.Status.IsDeleteMarker():
		return Retention{}, ErrMethodNotAllowed.New("querying retention data of delete marker is not allowed")
	case !info.Status.IsCommitted():
		return Retention{}, ErrMethodNotAllowed.New(noLockFromUncommittedErrMsg)
	}

	if err = info.Retention.Verify(); err != nil {
		return Retention{}, Error.Wrap(err)
	}

	return info.Retention, nil
}

// GetObjectExactVersionRetention returns the retention configuration of an exact version of an object.
func (s *SpannerAdapter) GetObjectExactVersionRetention(ctx context.Context, opts GetObjectExactVersionRetention) (_ Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT retention_mode, retain_until, status
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
			"version":     opts.Version,
		},
	}), func(row *spanner.Row, info *lockInfoAndStatus) error {
		err := row.Columns(lockModeWrapper{retentionMode: &info.Retention.Mode}, timeWrapper{&info.Retention.RetainUntil}, &info.Status)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Retention{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return Retention{}, Error.New("unable to query object retention configuration: %w", err)
	}

	switch {
	case info.Status.IsDeleteMarker():
		return Retention{}, ErrMethodNotAllowed.New("querying retention data of delete marker is not allowed")
	case !info.Status.IsCommitted():
		return Retention{}, ErrMethodNotAllowed.New(noLockFromUncommittedErrMsg)
	}

	if err = info.Retention.Verify(); err != nil {
		return Retention{}, Error.Wrap(err)
	}

	return info.Retention, nil
}

// GetObjectLastCommittedRetention contains arguments necessary for retrieving the retention
// configuration of the most recently committed version of an object.
type GetObjectLastCommittedRetention struct {
	ObjectLocation
}

// GetObjectLastCommittedRetention returns the retention configuration of the most recently
// committed version of an object.
func (db *DB) GetObjectLastCommittedRetention(ctx context.Context, opts GetObjectLastCommittedRetention) (retention Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Retention{}, err
	}

	retention, err = db.ChooseAdapter(opts.ProjectID).GetObjectLastCommittedRetention(ctx, opts)
	if err != nil {
		return Retention{}, err
	}

	return retention, nil
}

// GetObjectLastCommittedRetention returns the retention configuration of the most recently
// committed version of an object.
func (p *PostgresAdapter) GetObjectLastCommittedRetention(ctx context.Context, opts GetObjectLastCommittedRetention) (_ Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	var info lockInfoAndStatus

	err = p.db.QueryRowContext(ctx, `
		SELECT retention_mode, retain_until, status
		FROM objects
		WHERE
			(project_id, bucket_name, object_key) = ($1, $2, $3)
			AND status <> `+statusPending+`
		ORDER BY version DESC
		LIMIT 1
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey,
	).Scan(lockModeWrapper{retentionMode: &info.Retention.Mode}, timeWrapper{&info.Retention.RetainUntil}, &info.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Retention{}, ErrObjectNotFound.Wrap(Error.Wrap(err))
		}
		return Retention{}, Error.New("unable to query object retention configuration: %w", err)
	}
	if info.Status.IsDeleteMarker() {
		return Retention{}, ErrMethodNotAllowed.New("querying retention data of delete marker is not allowed")
	}
	if err = info.Retention.Verify(); err != nil {
		return Retention{}, Error.Wrap(err)
	}

	return info.Retention, nil
}

// GetObjectLastCommittedRetention returns the retention configuration of the most recently
// committed version of an object.
func (s *SpannerAdapter) GetObjectLastCommittedRetention(ctx context.Context, opts GetObjectLastCommittedRetention) (_ Retention, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := spannerutil.CollectRow(s.client.Single().Query(ctx, spanner.Statement{
		SQL: `
			SELECT retention_mode, retain_until, status
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
				AND status <> ` + statusPending + `
			ORDER BY version DESC
			LIMIT 1
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}), func(row *spanner.Row, info *lockInfoAndStatus) error {
		err := row.Columns(lockModeWrapper{retentionMode: &info.Retention.Mode}, timeWrapper{&info.Retention.RetainUntil}, &info.Status)
		if err != nil {
			return Error.Wrap(err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return Retention{}, ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
		}
		return Retention{}, Error.New("unable to query object retention configuration: %w", err)
	}

	if info.Status.IsDeleteMarker() {
		return Retention{}, ErrMethodNotAllowed.New("querying retention data of delete marker is not allowed")
	}
	if err = info.Retention.Verify(); err != nil {
		return Retention{}, Error.Wrap(err)
	}

	return info.Retention, nil
}

// TestingAllObjects gets all objects.
// Use only for testing purposes.
func (db *DB) TestingAllObjects(ctx context.Context) (objects []Object, err error) {
	defer mon.Task()(&ctx)(&err)

	var rawObjects []RawObject
	for _, a := range db.adapters {
		adapterObjects, err := a.TestingGetAllObjects(ctx)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		rawObjects = append(rawObjects, adapterObjects...)
	}
	sortRawObjects(rawObjects)
	objects = make([]Object, len(rawObjects))
	for i, o := range rawObjects {
		objects[i] = Object(o)
	}

	return objects, nil
}

// TestingAllSegments gets all segments.
// Use only for testing purposes.
func (db *DB) TestingAllSegments(ctx context.Context) (segments []Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	var rawSegments []RawSegment
	for _, a := range db.adapters {
		adapterSegments, err := a.TestingGetAllSegments(ctx, db.aliasCache)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		rawSegments = append(rawSegments, adapterSegments...)
	}
	sortRawSegments(rawSegments)
	for _, s := range rawSegments {
		segments = append(segments, Segment(s))
	}

	return segments, nil
}

type lockInfoAndStatus struct {
	Status    ObjectStatus
	Retention Retention
	LegalHold bool
}
