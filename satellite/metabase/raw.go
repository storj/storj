// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// RawObject defines the full object that is stored in the database. It should be rarely used directly.
type RawObject struct {
	ObjectStream

	CreatedAt time.Time
	ExpiresAt *time.Time

	Status       ObjectStatus
	SegmentCount int32

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	// TotalPlainSize is 0 for a migrated object.
	TotalPlainSize     int64
	TotalEncryptedSize int64
	// FixedSegmentSize is 0 for a migrated object.
	FixedSegmentSize int32

	Encryption storj.EncryptionParameters

	// ZombieDeletionDeadline defines when the pending raw object should be deleted from the database.
	// This is as a safeguard against objects that failed to upload and the client has not indicated
	// whether they want to continue uploading or delete the already uploaded data.
	ZombieDeletionDeadline *time.Time
}

// RawPendingObject defines the full pending object that is stored in the database. It should be rarely used directly.
type RawPendingObject struct {
	PendingObjectStream

	CreatedAt time.Time
	ExpiresAt *time.Time

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	Encryption storj.EncryptionParameters

	// ZombieDeletionDeadline defines when the pending raw object should be deleted from the database.
	// This is as a safeguard against objects that failed to upload and the client has not indicated
	// whether they want to continue uploading or delete the already uploaded data.
	ZombieDeletionDeadline *time.Time
}

// RawSegment defines the full segment that is stored in the database. It should be rarely used directly.
type RawSegment struct {
	StreamID uuid.UUID
	Position SegmentPosition

	CreatedAt  time.Time // non-nillable
	RepairedAt *time.Time
	ExpiresAt  *time.Time

	RootPieceID       storj.PieceID
	EncryptedKeyNonce []byte
	EncryptedKey      []byte

	EncryptedSize int32 // size of the whole segment (not a piece)
	// PlainSize is 0 for a migrated object.
	PlainSize int32
	// PlainOffset is 0 for a migrated object.
	PlainOffset   int64
	EncryptedETag []byte

	Redundancy storj.RedundancyScheme

	InlineData []byte
	Pieces     Pieces

	Placement storj.PlacementConstraint
}

// RawCopy contains a copy that is stored in the database.
type RawCopy struct {
	StreamID         uuid.UUID
	AncestorStreamID uuid.UUID
}

// RawState contains full state of a table.
type RawState struct {
	Objects        []RawObject
	PendingObjects []RawPendingObject
	Segments       []RawSegment
}

// TestingGetState returns the state of the database.
func (db *DB) TestingGetState(ctx context.Context) (_ *RawState, err error) {
	state := &RawState{}

	state.Objects, err = db.testingGetAllObjects(ctx)
	if err != nil {
		return nil, Error.New("GetState: %w", err)
	}

	state.PendingObjects, err = db.testingGetAllPendingObjects(ctx)
	if err != nil {
		return nil, Error.New("GetState: %w", err)
	}

	state.Segments, err = db.testingGetAllSegments(ctx)
	if err != nil {
		return nil, Error.New("GetState: %w", err)
	}

	return state, nil
}

// TestingDeleteAll deletes all objects and segments from the database.
func (db *DB) TestingDeleteAll(ctx context.Context) (err error) {
	_, err = db.db.ExecContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM objects;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM pending_objects;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM segments;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM node_aliases;
		WITH ignore_full_scan_for_test AS (SELECT 1) SELECT setval('node_alias_seq', 1, false);
	`)
	db.aliasCache = NewNodeAliasCache(db)
	return Error.Wrap(err)
}

// testingGetAllObjects returns the state of the database.
func (db *DB) testingGetAllObjects(ctx context.Context) (_ []RawObject, err error) {
	objs := []RawObject{}

	rows, err := db.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			zombie_deletion_deadline
		FROM objects
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
	`)
	if err != nil {
		return nil, Error.New("testingGetAllObjects query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var obj RawObject
		err := rows.Scan(
			&obj.ProjectID,
			&obj.BucketName,
			&obj.ObjectKey,
			&obj.Version,
			&obj.StreamID,

			&obj.CreatedAt,
			&obj.ExpiresAt,

			&obj.Status, // TODO: fix encoding
			&obj.SegmentCount,

			&obj.EncryptedMetadataNonce,
			&obj.EncryptedMetadata,
			&obj.EncryptedMetadataEncryptedKey,

			&obj.TotalPlainSize,
			&obj.TotalEncryptedSize,
			&obj.FixedSegmentSize,

			encryptionParameters{&obj.Encryption},
			&obj.ZombieDeletionDeadline,
		)
		if err != nil {
			return nil, Error.New("testingGetAllObjects scan failed: %w", err)
		}
		objs = append(objs, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("testingGetAllObjects scan failed: %w", err)
	}

	if len(objs) == 0 {
		return nil, nil
	}
	return objs, nil
}

// testingGetAllPendingObjects returns the state of the database.
func (db *DB) testingGetAllPendingObjects(ctx context.Context) (_ []RawPendingObject, err error) {
	objs := []RawPendingObject{}

	rows, err := db.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			project_id, bucket_name, object_key, stream_id,
			created_at, expires_at,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			encryption, zombie_deletion_deadline
		FROM pending_objects
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, stream_id ASC
	`)
	if err != nil {
		return nil, Error.New("testingGetAllPendingObjects query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var obj RawPendingObject
		err := rows.Scan(
			&obj.ProjectID,
			&obj.BucketName,
			&obj.ObjectKey,
			&obj.StreamID,

			&obj.CreatedAt,
			&obj.ExpiresAt,

			&obj.EncryptedMetadataNonce,
			&obj.EncryptedMetadata,
			&obj.EncryptedMetadataEncryptedKey,

			encryptionParameters{&obj.Encryption},
			&obj.ZombieDeletionDeadline,
		)
		if err != nil {
			return nil, Error.New("testingGetAllPendingObjects scan failed: %w", err)
		}
		objs = append(objs, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("testingGetAllPendingObjects scan failed: %w", err)
	}

	if len(objs) == 0 {
		return nil, nil
	}
	return objs, nil
}

// testingGetAllSegments returns the state of the database.
func (db *DB) testingGetAllSegments(ctx context.Context) (_ []RawSegment, err error) {
	segs := []RawSegment{}

	rows, err := db.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			stream_id, position,
			created_at, repaired_at, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size,
			plain_offset, plain_size,
			encrypted_etag,
			redundancy,
			inline_data, remote_alias_pieces,
			placement
		FROM segments
		ORDER BY stream_id ASC, position ASC
	`)
	if err != nil {
		return nil, Error.New("testingGetAllSegments query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var seg RawSegment
		var aliasPieces AliasPieces
		err := rows.Scan(
			&seg.StreamID,
			&seg.Position,

			&seg.CreatedAt,
			&seg.RepairedAt,
			&seg.ExpiresAt,

			&seg.RootPieceID,
			&seg.EncryptedKeyNonce,
			&seg.EncryptedKey,

			&seg.EncryptedSize,
			&seg.PlainOffset,
			&seg.PlainSize,
			&seg.EncryptedETag,

			redundancyScheme{&seg.Redundancy},

			&seg.InlineData,
			&aliasPieces,
			&seg.Placement,
		)
		if err != nil {
			return nil, Error.New("testingGetAllSegments scan failed: %w", err)
		}

		seg.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			return nil, Error.New("testingGetAllSegments convert aliases to pieces failed: %w", err)
		}

		segs = append(segs, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, Error.New("testingGetAllSegments scan failed: %w", err)
	}

	if len(segs) == 0 {
		return nil, nil
	}
	return segs, nil
}
