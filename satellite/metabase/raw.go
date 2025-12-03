// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/dbutil/spannerutil"
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
func (s *SpannerAdapter) TestingDeleteAll(ctx context.Context) (err error) {
	_, err = s.client.Apply(ctx, []*spanner.Mutation{
		spanner.Delete("objects", spanner.AllKeys()),
		spanner.Delete("segments", spanner.AllKeys()),
		spanner.Delete("node_aliases", spanner.AllKeys()),
	})
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
func (s *SpannerAdapter) TestingGetAllObjects(ctx context.Context) (_ []RawObject, err error) {
	return spannerutil.CollectRows(s.client.Single().Read(ctx, "objects", spanner.AllKeys(), []string{
		"project_id", "bucket_name", "object_key", "version", "stream_id",
		"created_at", "expires_at",
		"status", "segment_count",
		"encrypted_metadata_nonce", "encrypted_metadata", "encrypted_metadata_encrypted_key", "encrypted_etag",
		"total_plain_size", "total_encrypted_size", "fixed_segment_size",
		"encryption", "zombie_deletion_deadline", "retention_mode", "retain_until",
	}), func(row *spanner.Row, obj *RawObject) error {
		err := row.Columns(
			&obj.ProjectID,
			&obj.BucketName,
			&obj.ObjectKey,
			&obj.Version,
			&obj.StreamID,

			&obj.CreatedAt,
			&obj.ExpiresAt,

			&obj.Status,
			spannerutil.Int(&obj.SegmentCount),

			&obj.EncryptedMetadataNonce,
			&obj.EncryptedMetadata,
			&obj.EncryptedMetadataEncryptedKey,
			&obj.EncryptedETag,

			&obj.TotalPlainSize,
			&obj.TotalEncryptedSize,
			spannerutil.Int(&obj.FixedSegmentSize),

			&obj.Encryption,
			&obj.ZombieDeletionDeadline,
			lockModeWrapper{
				retentionMode: &obj.Retention.Mode,
				legalHold:     &obj.LegalHold,
			},
			timeWrapper{&obj.Retention.RetainUntil},
		)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = obj.Retention.Verify(); err != nil {
			return Error.Wrap(err)
		}

		return Error.Wrap(err)
	})
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
func (s *SpannerAdapter) TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error) {
	const maxRowsPerBatch = 250000

	progress, total := 0, len(objects)
	for len(objects) > 0 {
		batch := objects
		if len(batch) > maxRowsPerBatch {
			batch = batch[:maxRowsPerBatch]
		}
		objects = objects[len(batch):]

		source := newCopyFromRawObjects(batch)
		muts := make([]*spanner.Mutation, 0, len(batch))
		for source.Next() {
			vals, err := source.Values()
			if err != nil {
				return Error.Wrap(err)
			}
			// Change the int32s to int64s to appease the capricious gods of Spanner.
			for i := range vals {
				if v, ok := vals[i].(int32); ok {
					vals[i] = int64(v)
				}
			}
			muts = append(muts, spanner.Insert("objects", source.Columns(), vals))
		}
		_, err = s.client.Apply(ctx, muts, spanner.TransactionTag("testing-batch-insert-objects"))
		if err != nil {
			return Error.Wrap(err)
		}

		progress += len(batch)
		s.log.Info("batch insert", zap.Int("progress", progress), zap.Int("total", total))
	}
	return nil
}

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

func (ctr *copyFromRawObjects) Columns() []string {
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

		"total_plain_size",
		"total_encrypted_size",
		"fixed_segment_size",

		"encryption",
		"zombie_deletion_deadline",
	}
}

func (ctr *copyFromRawObjects) Values() ([]any, error) {
	obj := &ctr.rows[ctr.idx]
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

		obj.TotalPlainSize,
		obj.TotalEncryptedSize,
		obj.FixedSegmentSize,

		&obj.Encryption,
		obj.ZombieDeletionDeadline,
	}, nil
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
func (s *SpannerAdapter) TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (segments []RawSegment, err error) {
	return spannerutil.CollectRows(s.client.Single().Read(ctx, "segments", spanner.AllKeys(), []string{
		"stream_id", "position",
		"created_at", "repaired_at", "expires_at",
		"root_piece_id", "encrypted_key_nonce", "encrypted_key",
		"encrypted_size", "plain_offset", "plain_size",
		"encrypted_etag", "redundancy", "inline_data", "remote_alias_pieces",
		"placement",
	}), func(row *spanner.Row, segment *RawSegment) error {
		var aliasPieces AliasPieces

		err := row.Columns(
			&segment.StreamID, &segment.Position,
			&segment.CreatedAt, &segment.RepairedAt, &segment.ExpiresAt,
			&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			spannerutil.Int(&segment.EncryptedSize), &segment.PlainOffset, spannerutil.Int(&segment.PlainSize),
			&segment.EncryptedETag,
			&segment.Redundancy,
			&segment.InlineData, &aliasPieces,
			&segment.Placement,
		)
		if err != nil {
			return Error.Wrap(err)
		}

		segment.Pieces, err = aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
		if err != nil {
			return Error.New("convert aliases to pieces failed: %w", err)
		}

		return nil
	})
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

	"encrypted_size",
	"plain_size",
	"plain_offset",

	"redundancy",
	"inline_data",
	"remote_alias_pieces",
	"placement",
}

// spannerInsertSegment creates a spanner mutation for inserting the object.
func spannerInsertSegment(obj RawSegment, aliasPieces []byte) *spanner.Mutation {
	return spanner.Insert("segments", rawSegmentColumns, []any{
		obj.StreamID.Bytes(),
		int64(obj.Position.Encode()),

		obj.CreatedAt,
		obj.RepairedAt,
		obj.ExpiresAt,

		obj.RootPieceID.Bytes(),
		obj.EncryptedKeyNonce,
		obj.EncryptedKey,
		obj.EncryptedETag,

		int64(obj.EncryptedSize),
		int64(obj.PlainSize),
		obj.PlainOffset,

		obj.Redundancy,
		obj.InlineData,
		aliasPieces,
		obj.Placement,
	})
}

type copyFromRawSegments struct {
	idx     int
	rows    []RawSegment
	aliases []AliasPieces
	row     []any
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

func (ctr *copyFromRawSegments) Columns() []string {
	return rawSegmentColumns
}

func (ctr *copyFromRawSegments) Values() ([]any, error) {
	obj := &ctr.rows[ctr.idx]
	aliases := &ctr.aliases[ctr.idx]

	aliasPieces, err := aliases.Bytes()
	if err != nil {
		return nil, err
	}
	ctr.row = append(ctr.row[:0],
		obj.StreamID.Bytes(),
		obj.Position.Encode(),

		obj.CreatedAt,
		obj.RepairedAt,
		obj.ExpiresAt,

		obj.RootPieceID.Bytes(),
		obj.EncryptedKeyNonce,
		obj.EncryptedKey,
		obj.EncryptedETag,

		obj.EncryptedSize,
		obj.PlainSize,
		obj.PlainOffset,

		obj.Redundancy,
		obj.InlineData,
		aliasPieces,
		obj.Placement,
	)
	return ctr.row, nil
}

func (ctr *copyFromRawSegments) Err() error { return nil }

// TestingBatchInsertSegments implements SpannerAdapter.
func (s *SpannerAdapter) TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error) {
	mutations := make([]*spanner.Mutation, len(segments))
	for i, segment := range segments {
		aliasPieces, err := aliasCache.EnsurePiecesToAliases(ctx, segment.Pieces)
		if err != nil {
			return Error.Wrap(err)
		}

		// TODO(spanner) verify if casting is good
		vals := append([]interface{}{},
			segment.StreamID,
			segment.Position,

			segment.CreatedAt,
			segment.RepairedAt,
			segment.ExpiresAt,

			segment.RootPieceID,
			segment.EncryptedKeyNonce,
			segment.EncryptedKey,
			segment.EncryptedETag,

			int64(segment.EncryptedSize),
			int64(segment.PlainSize),
			segment.PlainOffset,

			segment.Redundancy,
			segment.InlineData,
			aliasPieces,
			int64(segment.Placement),
		)

		mutations[i] = spanner.InsertOrUpdate("segments", rawSegmentColumns, vals)
	}

	_, err = s.client.Apply(ctx, mutations, spanner.TransactionTag("testing-batch-insert-segments"))
	return Error.Wrap(err)
}

// TestingSetObjectVersion sets the version of the object to the given value.
func (db *DB) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error) {
	return db.ChooseAdapter(object.ProjectID).TestingSetObjectVersion(ctx, object, randomVersion)
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
func (s *SpannerAdapter) TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error) {
	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// Spanner doesn't support to update primary key columns, so we need to delete and insert the objects.
		// https://cloud.google.com/spanner/docs/reference/standard-sql/dml-syntax#update-statement
		deletedRows := tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: "DELETE FROM objects " +
				"WHERE project_id = @project_id AND " +
				"bucket_name = @bucket_name AND " +
				"object_key = @object_key AND " +
				"stream_id = @stream_id " +
				"THEN RETURN *",
			Params: map[string]interface{}{
				"project_id":  object.ProjectID,
				"bucket_name": object.BucketName,
				"object_key":  object.ObjectKey,
				"stream_id":   object.StreamID,
			},
		},
			spanner.QueryOptions{RequestTag: "testing-set-object-version"},
		)

		deleteObjsNames := []string{}
		deleteObjsVals := []any{}

		defer deletedRows.Stop()
		for {
			row, err := deletedRows.Next()
			if err != nil {
				if errors.Is(err, iterator.Done) {
					break
				}

				return err
			}

			ncols := row.Size()
			for c := 0; c < ncols; c++ {
				name := row.ColumnName(c)

				if len(deleteObjsNames) < ncols {
					deleteObjsNames = append(deleteObjsNames, name)
				}

				if name == "version" {
					deleteObjsVals = append(deleteObjsVals, randomVersion)
					continue
				}

				var value spanner.GenericColumnValue
				if err := row.Column(c, &value); err != nil {
					return err
				}

				deleteObjsVals = append(deleteObjsVals, value)
			}

			if err := tx.BufferWrite([]*spanner.Mutation{
				spanner.Insert("objects", deleteObjsNames, deleteObjsVals),
			}); err != nil {
				return err
			}

			rowsAffected++
			// Reuse the allocated slice for the next iteration.
			deleteObjsVals = deleteObjsVals[:0]
		}

		return nil
	}, spanner.TransactionOptions{
		TransactionTag:              "testing-set-object-version",
		ExcludeTxnFromChangeStreams: true,
	})
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
func (s *SpannerAdapter) TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) (err error) {
	_, err = s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		_, err := tx.UpdateWithOptions(ctx, spanner.Statement{
			SQL:    "UPDATE segments SET placement = @placement WHERE true",
			Params: map[string]interface{}{"placement": placement},
		}, spanner.QueryOptions{RequestTag: "testing-set-placement-all-segments"})
		return err
	}, spanner.TransactionOptions{
		TransactionTag:              "testing-set-placement-all-segments",
		ExcludeTxnFromChangeStreams: true,
	})
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

	"total_plain_size",
	"total_encrypted_size",
	"fixed_segment_size",

	"encryption",
	"zombie_deletion_deadline",

	"retention_mode",
	"retain_until",
}

var spannerObjectColumns = sync.OnceValue(func() string {
	return strings.Join(rawObjectColumns, ", ")
})

// spannerInsertObject creates a spanner mutation for inserting the object.
func spannerInsertObject(obj RawObject) *spanner.Mutation {
	return spanner.Insert("objects", rawObjectColumns, spannerObjectArguments(obj))
}

// spannerInsertOrUpdateObject creates a spanner mutation for inserting or updaing the object.
func spannerInsertOrUpdateObject(obj RawObject) *spanner.Mutation {
	return spanner.InsertOrUpdate("objects", rawObjectColumns, spannerObjectArguments(obj))
}

func spannerObjectArguments(obj RawObject) []any {
	return []any{
		obj.ProjectID.Bytes(),
		obj.BucketName,
		[]byte(obj.ObjectKey),
		obj.Version,
		obj.StreamID.Bytes(),

		obj.CreatedAt,
		obj.ExpiresAt,

		obj.Status,
		int64(obj.SegmentCount),

		obj.EncryptedMetadataNonce,
		obj.EncryptedMetadata,
		obj.EncryptedMetadataEncryptedKey,
		obj.EncryptedETag,

		obj.TotalPlainSize,
		obj.TotalEncryptedSize,
		int64(obj.FixedSegmentSize),

		&obj.Encryption,
		obj.ZombieDeletionDeadline,

		lockModeWrapper{
			retentionMode: &obj.Retention.Mode,
			legalHold:     &obj.LegalHold,
		},
		timeWrapper{&obj.Retention.RetainUntil},
	}
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
