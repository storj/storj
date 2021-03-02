// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
)

const loopIteratorBatchSizeLimit = 2500

// LoopObjectEntry contains information about object needed by metainfo loop.
type LoopObjectEntry struct {
	ObjectStream                     // metrics, repair, tally
	ExpiresAt             *time.Time // tally
	SegmentCount          int32      // metrics
	EncryptedMetadataSize int        // tally
}

// LoopObjectsIterator iterates over a sequence of LoopObjectEntry items.
type LoopObjectsIterator interface {
	Next(ctx context.Context, item *LoopObjectEntry) bool
}

// IterateLoopObjects contains arguments necessary for listing objects in metabase.
type IterateLoopObjects struct {
	BatchSize int
}

// Verify verifies get object request fields.
func (opts *IterateLoopObjects) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// IterateLoopObjects iterates through all objects in metabase.
func (db *DB) IterateLoopObjects(ctx context.Context, opts IterateLoopObjects, fn func(context.Context, LoopObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	it := &loopIterator{
		db: db,

		batchSize: opts.BatchSize,

		curIndex: 0,
		cursor:   loopIterateCursor{},
	}

	// ensure batch size is reasonable
	if it.batchSize <= 0 || it.batchSize > loopIteratorBatchSizeLimit {
		it.batchSize = loopIteratorBatchSizeLimit
	}

	it.curRows, err = it.doNextQuery(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if rowsErr := it.curRows.Err(); rowsErr != nil {
			err = errs.Combine(err, rowsErr)
		}
		err = errs.Combine(err, it.curRows.Close())
	}()

	return fn(ctx, it)
}

// loopIterator enables iteration of all objects in metabase.
type loopIterator struct {
	db *DB

	batchSize int

	curIndex int
	curRows  tagsql.Rows
	cursor   loopIterateCursor
}

type loopIterateCursor struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	Version    Version
}

// Next returns true if there was another item and copy it in item.
func (it *loopIterator) Next(ctx context.Context, item *LoopObjectEntry) bool {
	next := it.curRows.Next()
	if !next {
		if it.curIndex < it.batchSize {
			return false
		}

		if it.curRows.Err() != nil {
			return false
		}

		rows, err := it.doNextQuery(ctx)
		if err != nil {
			return false
		}

		if it.curRows.Close() != nil {
			_ = rows.Close()
			return false
		}

		it.curRows = rows
		it.curIndex = 0
		if !it.curRows.Next() {
			return false
		}
	}

	err := it.scanItem(item)
	if err != nil {
		return false
	}

	it.curIndex++
	it.cursor.ProjectID = item.ProjectID
	it.cursor.BucketName = item.BucketName
	it.cursor.ObjectKey = item.ObjectKey
	it.cursor.Version = item.Version

	return true
}

func (it *loopIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.Query(ctx, `
		SELECT
			project_id, bucket_name,
			object_key, stream_id, version,
			expires_at,
			segment_count,
			LENGTH(COALESCE(encrypted_metadata,''))
		FROM objects
		WHERE (project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
		LIMIT $5
		`, it.cursor.ProjectID, []byte(it.cursor.BucketName),
		[]byte(it.cursor.ObjectKey), int(it.cursor.Version),
		it.batchSize,
	)
}

// scanItem scans doNextQuery results into LoopObjectEntry.
func (it *loopIterator) scanItem(item *LoopObjectEntry) error {
	return it.curRows.Scan(
		&item.ProjectID, &item.BucketName,
		&item.ObjectKey, &item.StreamID, &item.Version,
		&item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataSize,
	)
}

// LoopSegmentEntry contains information about segment metadata needed by metainfo loop.
type LoopSegmentEntry struct {
	StreamID      uuid.UUID
	Position      SegmentPosition
	RootPieceID   storj.PieceID
	EncryptedSize int32 // size of the whole segment (not a piece)
	Redundancy    storj.RedundancyScheme
	Pieces        Pieces
}

// Inline returns true if segment is inline.
func (s LoopSegmentEntry) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// ListLoopSegmentEntries contains arguments necessary for listing streams loop segment entries.
type ListLoopSegmentEntries struct {
	StreamIDs []uuid.UUID
}

// ListLoopSegmentEntriesResult result of listing streams loop segment entries.
type ListLoopSegmentEntriesResult struct {
	Segments []LoopSegmentEntry
}

// ListLoopSegmentEntries lists streams loop segment entries.
func (db *DB) ListLoopSegmentEntries(ctx context.Context, opts ListLoopSegmentEntries) (result ListLoopSegmentEntriesResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(opts.StreamIDs) == 0 {
		return ListLoopSegmentEntriesResult{}, ErrInvalidRequest.New("StreamIDs list is empty")
	}

	// TODO do something like pgutil.UUIDArray()
	ids := make([][]byte, len(opts.StreamIDs))
	for i, streamID := range opts.StreamIDs {
		if streamID.IsZero() {
			return ListLoopSegmentEntriesResult{}, ErrInvalidRequest.New("StreamID missing: index %d", i)
		}

		id := streamID
		ids[i] = id[:]
	}

	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i], ids[j]) < 0
	})

	err = withRows(db.db.Query(ctx, `
		SELECT
			stream_id, position,
			root_piece_id,
			encrypted_size,
			redundancy,
			remote_alias_pieces
		FROM segments
		WHERE
		    -- this turns out to be a little bit faster than stream_id IN (SELECT unnest($1::BYTEA[]))
			stream_id = ANY ($1::BYTEA[])
		ORDER BY stream_id ASC, position ASC
	`, pgutil.ByteaArray(ids)))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment LoopSegmentEntry
			var aliasPieces AliasPieces
			err = rows.Scan(
				&segment.StreamID, &segment.Position,
				&segment.RootPieceID,
				&segment.EncryptedSize,
				redundancyScheme{&segment.Redundancy},
				&aliasPieces,
			)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				return Error.New("failed to convert aliases to pieces: %w", err)
			}

			result.Segments = append(result.Segments, segment)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListLoopSegmentEntriesResult{}, nil
		}
		return ListLoopSegmentEntriesResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	return result, nil
}
