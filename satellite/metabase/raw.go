// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/tagsql"
)

// RawObject defines the full object that is stored in the database. It should be rarely used directly.
type RawObject struct {
	ObjectStream

	CreatedAt time.Time
	ExpiresAt *time.Time

	Status       ObjectStatus
	SegmentCount int32

	EncryptedUserData

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

	Retention Retention
	LegalHold bool
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
	PlainOffset int64

	EncryptedETag     []byte
	EncryptedChecksum []byte

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
	Objects  []RawObject
	Segments []RawSegment
}

func sortRawObjects(objects []RawObject) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].ObjectStream.Less(objects[j].ObjectStream)
	})
}

func sortRawSegments(segments []RawSegment) {
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StreamID == segments[j].StreamID {
			return segments[i].Position.Less(segments[j].Position)
		}
		return segments[i].StreamID.Less(segments[j].StreamID)
	})
}

// TestingGetState returns the state of the database.
func (db *DB) TestingGetState(ctx context.Context) (_ *RawState, err error) {
	state := &RawState{}

	for _, a := range db.adapters {
		objects, err := a.TestingGetAllObjects(ctx)
		if err != nil {
			return nil, Error.New("GetState: %w", err)
		}
		state.Objects = append(state.Objects, objects...)

		segments, err := a.TestingGetAllSegments(ctx, db.aliasCache)
		if err != nil {
			return nil, Error.New("GetState: %w", err)
		}
		state.Segments = append(state.Segments, segments...)
	}
	sortRawObjects(state.Objects)
	sortRawSegments(state.Segments)

	return state, nil
}

// TestingDeleteAll deletes all objects and segments from the database.
func (db *DB) TestingDeleteAll(ctx context.Context) (err error) {
	db.aliasCache = NewNodeAliasCache(db, db.aliasCache.fullRefresh)
	for _, a := range db.adapters {
		if err := a.TestingDeleteAll(ctx); err != nil {
			return err
		}
	}
	return nil
}

// TestingDeleteAll implements Adapter.
func (p *PostgresAdapter) TestingDeleteAll(ctx context.Context) (err error) {
	_, err = p.db.ExecContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM objects;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM segments;
		WITH ignore_full_scan_for_test AS (SELECT 1) DELETE FROM node_aliases;
		WITH ignore_full_scan_for_test AS (SELECT 1) SELECT setval('node_alias_seq', 1, false);
	`)
	return Error.Wrap(err)
}

// TestingDeleteAll implements Adapter.
func (t *TiDBAdapter) TestingDeleteAll(ctx context.Context) (err error) {
	// Avoid TRUNCATE: it's DDL in TiDB, bumps the schema version, and causes
	// "Information schema is changed" retries that stall concurrent INSERTs in
	// parallel tests. DELETE is plain DML. The node_aliases AUTO_INCREMENT
	// isn't reset (unlike Postgres's setval); the alias cache is recreated by
	// the caller, and tests don't depend on specific alias values.
	_, err = t.db.ExecContext(ctx, `DELETE FROM objects; DELETE FROM segments; DELETE FROM node_aliases; DELETE FROM bucket_eventing_outbox;`)
	return Error.Wrap(err)
}

// TestingGetAllObjects returns the state of the database.
func (p *PostgresAdapter) TestingGetAllObjects(ctx context.Context) (_ []RawObject, err error) {
	objs := []RawObject{}

	rows, err := p.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
			checksum,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			zombie_deletion_deadline,
			retention_mode, retain_until
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
			&obj.EncryptedETag,
			&obj.Checksum,

			&obj.TotalPlainSize,
			&obj.TotalEncryptedSize,
			&obj.FixedSegmentSize,

			&obj.Encryption,
			&obj.ZombieDeletionDeadline,
			lockModeWrapper{
				retentionMode: &obj.Retention.Mode,
				legalHold:     &obj.LegalHold,
			},
			timeWrapper{&obj.Retention.RetainUntil},
		)
		if err != nil {
			return nil, Error.New("testingGetAllObjects scan failed: %w", err)
		}

		if err = obj.Retention.Verify(); err != nil {
			return nil, Error.Wrap(err)
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

// TestingGetAllObjects returns the state of the database.
func (t *TiDBAdapter) TestingGetAllObjects(ctx context.Context) (_ []RawObject, err error) {
	objs := []RawObject{}

	rows, err := t.db.QueryContext(ctx, `
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
			checksum,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption,
			zombie_deletion_deadline,
			retention_mode, retain_until
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

			&obj.Status,
			&obj.SegmentCount,

			&obj.EncryptedMetadataNonce,
			&obj.EncryptedMetadata,
			&obj.EncryptedMetadataEncryptedKey,
			&obj.EncryptedETag,
			&obj.Checksum,

			&obj.TotalPlainSize,
			&obj.TotalEncryptedSize,
			&obj.FixedSegmentSize,

			&obj.Encryption,
			&obj.ZombieDeletionDeadline,
			lockModeWrapper{
				retentionMode: &obj.Retention.Mode,
				legalHold:     &obj.LegalHold,
			},
			timeWrapper{&obj.Retention.RetainUntil},
		)
		if err != nil {
			return nil, Error.New("testingGetAllObjects scan failed: %w", err)
		}

		if err = obj.Retention.Verify(); err != nil {
			return nil, Error.Wrap(err)
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

// TestingBatchInsertObjects batch inserts objects for testing.
// This implementation does no verification on the correctness of objects.
func (db *DB) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error) {
	objectsByAdapterType := make(map[reflect.Type][]RawObject)
	for _, obj := range objects {
		if obj.Status == 0 {
			return Error.New("object status not set")
		}
		if obj.Version == 0 {
			return Error.New("object version not set")
		}
		adapter := db.ChooseAdapter(obj.ProjectID)
		adapterType := reflect.TypeOf(adapter)
		objectsByAdapterType[adapterType] = append(objectsByAdapterType[adapterType], obj)
	}
	for _, adapter := range db.adapters {
		adapterType := reflect.TypeOf(adapter)
		err := adapter.TestingBatchInsertObjects(ctx, objectsByAdapterType[adapterType])
		if err != nil {
			return Error.Wrap(err)
		}
		delete(objectsByAdapterType, adapterType)
	}
	return nil
}

// TestingBatchInsertObjects batch inserts objects for testing.
func (p *PostgresAdapter) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error) {
	const maxRowsPerCopy = 250000

	return Error.Wrap(pgxutil.Conn(ctx, p.db,
		func(conn *pgx.Conn) error {
			progress, total := 0, len(objects)
			for len(objects) > 0 {
				batch := objects
				if len(batch) > maxRowsPerCopy {
					batch = batch[:maxRowsPerCopy]
				}
				objects = objects[len(batch):]

				source := newCopyFromRawObjects(batch)
				_, err := conn.CopyFrom(ctx, pgx.Identifier{"objects"}, source.Columns(), source)
				if err != nil {
					return err
				}

				progress += len(batch)
				p.log.Info("batch insert", zap.Int("progress", progress), zap.Int("total", total))
			}
			return err
		}))
}

// TestingBatchInsertObjects batch inserts objects for testing.
func (t *TiDBAdapter) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error) {
	const maxRowsPerBatch = 1000

	cols := objectInsertColumns()

	for start, batch := range batched(objects, maxRowsPerBatch) {
		args := make([]any, 0, len(batch)*len(cols))
		for i := range batch {
			args = append(args, objectInsertValues(&batch[i])...)
		}

		query := tidbBatchInsertQuery("objects", cols, len(batch))
		if _, err := t.db.ExecContext(ctx, query, args...); err != nil {
			return Error.Wrap(err)
		}

		t.log.Info("batch insert", zap.Int("progress", start+len(batch)), zap.Int("total", len(objects)))
	}
	return nil
}

// objectInsertColumns returns the column names written by the
// TestingBatchInsertObjects code paths in the order produced by
// objectInsertValues.
//
// NOTE: This intentionally omits retention_mode/retain_until — the existing
// testing batch-insert path predates those columns. The package-level
// rawObjectColumns includes them.
func objectInsertColumns() []string {
	return []string{
		"project_id",
		"bucket_name",
		"object_key",
		"version",
		"stream_id",

		"created_at",
		"expires_at",

		"status",
		"segment_count",

		"encrypted_metadata_nonce",
		"encrypted_metadata",
		"encrypted_metadata_encrypted_key",
		"encrypted_etag",
		"checksum",

		"total_plain_size",
		"total_encrypted_size",
		"fixed_segment_size",

		"encryption",
		"zombie_deletion_deadline",
	}
}

// objectInsertValues returns the column values of obj in the order returned by
// objectInsertColumns.
func objectInsertValues(obj *RawObject) []any {
	return []any{
		obj.ProjectID.Bytes(),
		obj.BucketName,
		[]byte(obj.ObjectKey),
		obj.Version,
		obj.StreamID.Bytes(),

		obj.CreatedAt,
		obj.ExpiresAt,

		obj.Status, // TODO: fix encoding
		obj.SegmentCount,

		obj.EncryptedMetadataNonce,
		obj.EncryptedMetadata,
		obj.EncryptedMetadataEncryptedKey,
		obj.EncryptedETag,
		obj.Checksum,

		obj.TotalPlainSize,
		obj.TotalEncryptedSize,
		obj.FixedSegmentSize,

		&obj.Encryption,
		obj.ZombieDeletionDeadline,
	}
}

// copyFromRawObjects adapts a slice of RawObject to the pgx.CopyFromSource
// interface used by the Postgres adapter.
type copyFromRawObjects struct {
	idx  int
	rows []RawObject
}

func newCopyFromRawObjects(rows []RawObject) *copyFromRawObjects {
	return &copyFromRawObjects{
		rows: rows,
		idx:  -1,
	}
}

func (ctr *copyFromRawObjects) Next() bool {
	ctr.idx++
	return ctr.idx < len(ctr.rows)
}

func (ctr *copyFromRawObjects) Columns() []string { return objectInsertColumns() }

func (ctr *copyFromRawObjects) Values() ([]any, error) {
	return objectInsertValues(&ctr.rows[ctr.idx]), nil
}

func (ctr *copyFromRawObjects) Err() error { return nil }

// TestingGetAllSegments implements Adapter.
func (p *PostgresAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error) {
	segs := []RawSegment{}

	rows, err := p.db.QueryContext(ctx, `
		WITH ignore_full_scan_for_test AS (SELECT 1)
		SELECT
			stream_id, position,
			created_at, repaired_at, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size,
			plain_offset, plain_size,
			encrypted_etag, encrypted_checksum,
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
			&seg.EncryptedChecksum,

			&seg.Redundancy,

			&seg.InlineData,
			&aliasPieces,
			&seg.Placement,
		)
		if err != nil {
			return nil, Error.New("testingGetAllSegments scan failed: %w", err)
		}

		seg.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
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

// TestingGetAllSegments implements Adapter.
func (t *TiDBAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error) {
	segs := []RawSegment{}

	rows, err := t.db.QueryContext(ctx, `
		SELECT
			stream_id, position,
			created_at, repaired_at, expires_at,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size,
			plain_offset, plain_size,
			encrypted_etag, encrypted_checksum,
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
			&seg.EncryptedChecksum,

			&seg.Redundancy,

			&seg.InlineData,
			&aliasPieces,
			&seg.Placement,
		)
		if err != nil {
			return nil, Error.New("testingGetAllSegments scan failed: %w", err)
		}

		seg.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
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

// TestingBatchInsertSegments batch inserts segments for testing.
// This implementation does no verification on the correctness of segments.
func (db *DB) TestingBatchInsertSegments(ctx context.Context, segments []RawSegment) (err error) {
	return db.ChooseAdapter(uuid.UUID{}).TestingBatchInsertSegments(ctx, db.aliasCache, segments)
}

// TestingBatchInsertSegments implements postgres adapter.
func (p *PostgresAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	const maxRowsPerCopy = 250000

	minLength := len(segments)
	if maxRowsPerCopy < minLength {
		minLength = maxRowsPerCopy
	}

	aliases := make([]AliasPieces, 0, minLength)
	return Error.Wrap(pgxutil.Conn(ctx, p.db,
		func(conn *pgx.Conn) error {
			progress, total := 0, len(segments)
			for len(segments) > 0 {
				batch := segments
				if len(batch) > maxRowsPerCopy {
					batch = batch[:maxRowsPerCopy]
				}
				segments = segments[len(batch):]

				aliases = aliases[:len(batch)]
				for i, segment := range batch {
					aliases[i], err = aliasCache.EnsurePiecesToAliases(ctx, segment.Pieces)
					if err != nil {
						return err
					}
				}

				source := newCopyFromRawSegments(batch, aliases)
				_, err := conn.CopyFrom(ctx, pgx.Identifier{"segments"}, source.Columns(), source)
				if err != nil {
					return err
				}

				progress += len(batch)
				p.log.Info("batch insert", zap.Int("progress", progress), zap.Int("total", total))
			}
			return err
		}))
}

var rawSegmentColumns = []string{
	"stream_id",
	"position",

	"created_at",
	"repaired_at",
	"expires_at",

	"root_piece_id",
	"encrypted_key_nonce",
	"encrypted_key",
	"encrypted_etag",
	"encrypted_checksum",

	"encrypted_size",
	"plain_size",
	"plain_offset",

	"redundancy",
	"inline_data",
	"remote_alias_pieces",
	"placement",
}

// segmentInsertValues returns the column values of segment in the order
// defined by rawSegmentColumns. aliasPieces must already be encoded.
func segmentInsertValues(segment *RawSegment, aliasPieces []byte) []any {
	return []any{
		segment.StreamID.Bytes(),
		segment.Position.Encode(),

		segment.CreatedAt,
		segment.RepairedAt,
		segment.ExpiresAt,

		segment.RootPieceID.Bytes(),
		segment.EncryptedKeyNonce,
		segment.EncryptedKey,
		segment.EncryptedETag,
		segment.EncryptedChecksum,

		segment.EncryptedSize,
		segment.PlainSize,
		segment.PlainOffset,

		segment.Redundancy,
		segment.InlineData,
		aliasPieces,
		segment.Placement,
	}
}

// copyFromRawSegments adapts a slice of RawSegment to the pgx.CopyFromSource
// interface used by the Postgres adapter.
type copyFromRawSegments struct {
	idx     int
	rows    []RawSegment
	aliases []AliasPieces
}

func newCopyFromRawSegments(rows []RawSegment, aliases []AliasPieces) *copyFromRawSegments {
	return &copyFromRawSegments{
		rows:    rows,
		aliases: aliases,
		idx:     -1,
	}
}

func (ctr *copyFromRawSegments) Next() bool {
	ctr.idx++
	return ctr.idx < len(ctr.rows)
}

func (ctr *copyFromRawSegments) Columns() []string { return rawSegmentColumns }

func (ctr *copyFromRawSegments) Values() ([]any, error) {
	aliasPieces, err := ctr.aliases[ctr.idx].Bytes()
	if err != nil {
		return nil, err
	}
	return segmentInsertValues(&ctr.rows[ctr.idx], aliasPieces), nil
}

func (ctr *copyFromRawSegments) Err() error { return nil }

// TestingBatchInsertSegments implements TiDBAdapter.
func (t *TiDBAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	const maxRowsPerBatch = 1000

	for start, batch := range batched(segments, maxRowsPerBatch) {
		args := make([]any, 0, len(batch)*len(rawSegmentColumns))
		for i := range batch {
			aliases, err := aliasCache.EnsurePiecesToAliases(ctx, batch[i].Pieces)
			if err != nil {
				return Error.Wrap(err)
			}
			aliasPieces, err := aliases.Bytes()
			if err != nil {
				return Error.Wrap(err)
			}
			args = append(args, segmentInsertValues(&batch[i], aliasPieces)...)
		}

		query := tidbBatchInsertQuery("segments", rawSegmentColumns, len(batch))
		if _, err := t.db.ExecContext(ctx, query, args...); err != nil {
			return Error.Wrap(err)
		}

		t.log.Info("batch insert", zap.Int("progress", start+len(batch)), zap.Int("total", len(segments)))
	}
	return nil
}

// TestingSetObjectVersion sets the version of the object to the given value.
func (db *DB) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error) {
	return db.ChooseAdapter(object.ProjectID).TestingSetObjectVersion(ctx, object, randomVersion)
}

// TestingSetObjectCreatedAt sets the created_at of the object to the given value in tests.
func (db *DB) TestingSetObjectCreatedAt(ctx context.Context, object ObjectStream, createdAt time.Time) (rowsAffected int64, err error) {
	return db.ChooseAdapter(object.ProjectID).TestingSetObjectCreatedAt(ctx, object, createdAt)
}

// TestingSetObjectVersion sets the version of the object to the given value.
func (p *PostgresAdapter) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error) {
	res, err := p.db.ExecContext(ctx,
		"UPDATE objects SET version = $1 WHERE project_id = $2 AND bucket_name = $3 AND object_key = $4 AND stream_id = $5",
		randomVersion, object.ProjectID, object.BucketName, object.ObjectKey, object.StreamID,
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	rowsAffected, err = res.RowsAffected()
	return rowsAffected, Error.Wrap(err)
}

// TestingSetObjectVersion sets the version of the object to the given value.
func (t *TiDBAdapter) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error) {
	res, err := t.db.ExecContext(ctx,
		"UPDATE objects SET version = ? WHERE (project_id, bucket_name, object_key, stream_id) = (?, ?, ?, ?)",
		randomVersion, object.ProjectID, object.BucketName, object.ObjectKey, object.StreamID,
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	rowsAffected, err = res.RowsAffected()
	return rowsAffected, Error.Wrap(err)
}

// TestingSetObjectCreatedAt sets the created_at of the object to the given value in tests.
func (p *PostgresAdapter) TestingSetObjectCreatedAt(ctx context.Context, object ObjectStream, createdAt time.Time) (rowsAffected int64, err error) {
	res, err := p.db.ExecContext(ctx,
		"UPDATE objects SET created_at = $1 WHERE project_id = $2 AND bucket_name = $3 AND object_key = $4 AND stream_id = $5",
		createdAt, object.ProjectID, object.BucketName, object.ObjectKey, object.StreamID,
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	rowsAffected, err = res.RowsAffected()
	return rowsAffected, Error.Wrap(err)
}

// TestingSetObjectCreatedAt sets the created_at of the object to the given value in tests.
func (t *TiDBAdapter) TestingSetObjectCreatedAt(ctx context.Context, object ObjectStream, createdAt time.Time) (rowsAffected int64, err error) {
	res, err := t.db.ExecContext(ctx,
		"UPDATE objects SET created_at = ? WHERE (project_id, bucket_name, object_key, stream_id) = (?, ?, ?, ?)",
		createdAt, object.ProjectID, object.BucketName, object.ObjectKey, object.StreamID,
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	rowsAffected, err = res.RowsAffected()
	return rowsAffected, Error.Wrap(err)
}

// TestingSetPlacementAllSegments sets the placement of all segments to the given value.
func (db *DB) TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) (err error) {
	for _, a := range db.adapters {
		err = a.TestingSetPlacementAllSegments(ctx, placement)
		if err != nil {
			return err
		}
	}
	return nil
}

// TestingSetPlacementAllSegments sets the placement of all segments to the given value.
func (p *PostgresAdapter) TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) (err error) {
	_, err = p.db.ExecContext(ctx, "UPDATE segments SET placement = $1", placement)
	return Error.Wrap(err)
}

// TestingSetPlacementAllSegments sets the placement of all segments to the given value.
func (t *TiDBAdapter) TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) (err error) {
	_, err = t.db.ExecContext(ctx, "UPDATE segments SET placement = ?", placement)
	return Error.Wrap(err)
}

var rawObjectColumns = []string{
	"project_id",
	"bucket_name",
	"object_key",
	"version",
	"stream_id",

	"created_at",
	"expires_at",

	"status",
	"segment_count",

	"encrypted_metadata_nonce",
	"encrypted_metadata",
	"encrypted_metadata_encrypted_key",
	"encrypted_etag",
	"checksum",

	"total_plain_size",
	"total_encrypted_size",
	"fixed_segment_size",

	"encryption",
	"zombie_deletion_deadline",

	"retention_mode",
	"retain_until",
}

var postgresObjectColumns = sync.OnceValue(func() string {
	return strings.Join(rawObjectColumns, ", ")
})

var postgresObjectInsertQuery = sync.OnceValue(func() string {
	postgresObjectColumns := strings.Join(rawObjectColumns, ", ")

	var args strings.Builder
	for i := range len(rawObjectColumns) {
		if i == 0 {
			fmt.Fprintf(&args, "$%v", i+1)
		} else {
			fmt.Fprintf(&args, ", $%v", i+1)
		}
	}

	return `INSERT INTO objects (` + postgresObjectColumns + `) SELECT ` + args.String()
})

var postgresObjectInsertOrUpdateQuery = sync.OnceValue(func() string {
	postgresObjectColumns := strings.Join(rawObjectColumns, ", ")

	var args strings.Builder
	for i := range len(rawObjectColumns) {
		if i == 0 {
			fmt.Fprintf(&args, "$%v", i+1)
		} else {
			fmt.Fprintf(&args, ", $%v", i+1)
		}
	}

	var updates strings.Builder
	// Skip the primary key columns (project_id, bucket_name, object_key, version)
	for i := 4; i < len(rawObjectColumns); i++ {
		if i > 4 {
			updates.WriteString(", ")
		}
		fmt.Fprintf(&updates, "%s = EXCLUDED.%s", rawObjectColumns[i], rawObjectColumns[i])
	}

	return `INSERT INTO objects (` + postgresObjectColumns + `) SELECT ` + args.String() +
		` ON CONFLICT (project_id, bucket_name, object_key, version) DO UPDATE SET ` + updates.String()
})

func postgresInsertObject(ctx context.Context, tx tagsql.Tx, object *RawObject) error {
	_, err := tx.ExecContext(ctx, postgresObjectInsertQuery(), postgresObjectArguments(object)...)
	if err != nil {
		return err
	}
	return nil
}

func postgresInsertOrUpdateObject(ctx context.Context, tx tagsql.Tx, object *RawObject) error {
	_, err := tx.ExecContext(ctx, postgresObjectInsertOrUpdateQuery(), postgresObjectArguments(object)...)
	if err != nil {
		return err
	}
	return nil
}

var tidbObjectInsertQuery = sync.OnceValue(func() string {
	cols := strings.Join(rawObjectColumns, ", ")
	placeholders := strings.Repeat("?, ", len(rawObjectColumns)-1) + "?"
	return `INSERT INTO objects (` + cols + `) VALUES (` + placeholders + `)`
})

var tidbObjectInsertOrUpdateQuery = sync.OnceValue(func() string {
	cols := strings.Join(rawObjectColumns, ", ")
	placeholders := strings.Repeat("?, ", len(rawObjectColumns)-1) + "?"
	var updates strings.Builder
	for i := 4; i < len(rawObjectColumns); i++ {
		if i > 4 {
			updates.WriteString(", ")
		}
		fmt.Fprintf(&updates, "%s = VALUES(%s)", rawObjectColumns[i], rawObjectColumns[i])
	}
	return `INSERT INTO objects (` + cols + `) VALUES (` + placeholders + `) ON DUPLICATE KEY UPDATE ` + updates.String()
})

// tidbTruncateObjectTimes truncates obj's DATETIME(6) columns to microsecond
// resolution so the value persisted by TiDB (which rounds half-up on store)
// matches the value still held on the in-memory *Object the caller will
// return — otherwise a fresh CreatedAt with sub-microsecond bits ends up
// rounded on disk while the caller hands the un-rounded value to the client.
func tidbTruncateObjectTimes(obj *RawObject) {
	obj.CreatedAt = obj.CreatedAt.Truncate(time.Microsecond)
	if obj.ExpiresAt != nil {
		t := obj.ExpiresAt.Truncate(time.Microsecond)
		obj.ExpiresAt = &t
	}
	if obj.ZombieDeletionDeadline != nil {
		t := obj.ZombieDeletionDeadline.Truncate(time.Microsecond)
		obj.ZombieDeletionDeadline = &t
	}
	obj.Retention.RetainUntil = obj.Retention.RetainUntil.Truncate(time.Microsecond)
}

// tidbInsertObject inserts object. It mutates object's DATETIME(6) fields to
// match the persisted (microsecond-truncated) values; see tidbTruncateObjectTimes.
func tidbInsertObject(ctx context.Context, tx tagsql.ExecQueryer, object *RawObject) error {
	tidbTruncateObjectTimes(object)
	_, err := tx.ExecContext(ctx, tidbObjectInsertQuery(), postgresObjectArguments(object)...)
	return err
}

// tidbInsertOrUpdateObject inserts or updates object. It mutates object's
// DATETIME(6) fields to match the persisted values; see tidbTruncateObjectTimes.
func tidbInsertOrUpdateObject(ctx context.Context, tx tagsql.ExecQueryer, object *RawObject) error {
	_, err := tx.ExecContext(ctx, tidbObjectInsertOrUpdateQuery(), tidbInsertOrUpdateObjectArgs(object)...)
	return err
}

// tidbInsertOrUpdateObjectArgs returns the bound arguments for
// tidbObjectInsertOrUpdateQuery. It mutates object's DATETIME(6) fields to match
// the persisted values; see tidbTruncateObjectTimes.
func tidbInsertOrUpdateObjectArgs(object *RawObject) []any {
	tidbTruncateObjectTimes(object)
	return postgresObjectArguments(object)
}

var tidbObjectMoveQuery = sync.OnceValue(func() string {
	// SET clause spans every column from version onward — the (project_id,
	// bucket_name, object_key) prefix of the primary key doesn't change.
	var setParts strings.Builder
	for i, col := range rawObjectColumns[3:] {
		if i > 0 {
			setParts.WriteString(", ")
		}
		setParts.WriteString(col)
		setParts.WriteString(" = ?")
	}
	return `UPDATE objects SET ` + setParts.String() +
		` WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`
})

// tidbMoveObjectQuery builds the statement and arguments that rewrite the row
// identified by (object.ProjectID, object.BucketName, object.ObjectKey,
// initialVersion) with all fields from object, including a new version. TiDB
// internally implements UPDATE-of-PK as delete-then-insert on the clustered
// index, so the caller must guarantee no row already exists at the new key;
// otherwise the statement fails with "Duplicate entry". The statement is
// expected to affect exactly one row.
// It mutates object's DATETIME(6) fields to match the persisted values; see
// tidbTruncateObjectTimes.
func tidbMoveObjectQuery(object *RawObject, initialVersion Version) (statement string, args []any) {
	tidbTruncateObjectTimes(object)
	// postgresObjectArguments returns values in rawObjectColumns order; the
	// first three (project_id, bucket_name, object_key) match the WHERE
	// clause's literal columns and don't appear in the SET clause.
	setArgs := postgresObjectArguments(object)[3:]
	args = make([]any, 0, len(setArgs)+4)
	args = append(args, setArgs...)
	args = append(args, object.ProjectID.Bytes(), object.BucketName, object.ObjectKey, initialVersion)
	return tidbObjectMoveQuery(), args
}

func postgresObjectArguments(obj *RawObject) []any {
	return []any{
		obj.ProjectID.Bytes(),
		obj.BucketName,
		obj.ObjectKey,
		obj.Version,
		obj.StreamID.Bytes(),

		obj.CreatedAt,
		obj.ExpiresAt,

		obj.Status,
		obj.SegmentCount,

		obj.EncryptedMetadataNonce,
		obj.EncryptedMetadata,
		obj.EncryptedMetadataEncryptedKey,
		obj.EncryptedETag,
		obj.Checksum,

		obj.TotalPlainSize,
		obj.TotalEncryptedSize,
		obj.FixedSegmentSize,

		&obj.Encryption,
		obj.ZombieDeletionDeadline,

		lockModeWrapper{
			retentionMode: &obj.Retention.Mode,
			legalHold:     &obj.LegalHold,
		},
		timeWrapper{&obj.Retention.RetainUntil},
	}
}

func postgresObjectScan(obj *RawObject) []any {
	return []any{
		&obj.ProjectID,
		&obj.BucketName,
		&obj.ObjectKey,
		&obj.Version,
		&obj.StreamID,

		&obj.CreatedAt,
		&obj.ExpiresAt,

		&obj.Status,
		&obj.SegmentCount,

		&obj.EncryptedMetadataNonce,
		&obj.EncryptedMetadata,
		&obj.EncryptedMetadataEncryptedKey,
		&obj.EncryptedETag,
		&obj.Checksum,

		&obj.TotalPlainSize,
		&obj.TotalEncryptedSize,
		&obj.FixedSegmentSize,

		&obj.Encryption,
		&obj.ZombieDeletionDeadline,

		lockModeWrapper{
			retentionMode: &obj.Retention.Mode,
			legalHold:     &obj.LegalHold,
		},
		timeWrapper{&obj.Retention.RetainUntil},
	}
}
