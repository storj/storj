// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/tagsql"
)

const loopIteratorBatchSizeLimit = 2500

// IterateLoopObjects contains arguments necessary for listing objects in metabase.
type IterateLoopObjects struct {
	BatchSize int

	AsOfSystemTime time.Time
}

// Verify verifies get object request fields.
func (opts *IterateLoopObjects) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// LoopObjectsIterator iterates over a sequence of LoopObjectEntry items.
type LoopObjectsIterator interface {
	Next(ctx context.Context, item *LoopObjectEntry) bool
}

// LoopObjectEntry contains information about object needed by metainfo loop.
type LoopObjectEntry struct {
	ObjectStream                       // metrics, repair, tally
	Status                ObjectStatus // verify
	CreatedAt             time.Time    // temp used by metabase-createdat-migration
	ExpiresAt             *time.Time   // tally
	SegmentCount          int32        // metrics
	EncryptedMetadataSize int          // tally
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

		curIndex:       0,
		cursor:         loopIterateCursor{},
		asOfSystemTime: opts.AsOfSystemTime,
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

	batchSize      int
	asOfSystemTime time.Time

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

	var asOfSystemTime string
	if !it.asOfSystemTime.IsZero() && it.db.implementation == dbutil.Cockroach {
		asOfSystemTime = fmt.Sprintf(` AS OF SYSTEM TIME '%d' `, it.asOfSystemTime.UnixNano())
	}

	return it.db.db.Query(ctx, `
		SELECT
			project_id, bucket_name,
			object_key, stream_id, version,
			status,
			created_at, expires_at,
			segment_count,
			LENGTH(COALESCE(encrypted_metadata,''))
		FROM objects
		`+asOfSystemTime+`
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
		&item.Status,
		&item.CreatedAt, &item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataSize,
	)
}

// IterateLoopStreams contains arguments necessary for listing multiple streams segments.
type IterateLoopStreams struct {
	StreamIDs []uuid.UUID

	AsOfSystemTime time.Time
}

// SegmentIterator returns the next segment.
type SegmentIterator func(segment *LoopSegmentEntry) bool

// LoopSegmentEntry contains information about segment metadata needed by metainfo loop.
type LoopSegmentEntry struct {
	StreamID      uuid.UUID
	Position      SegmentPosition
	CreatedAt     *time.Time // repair
	RepairedAt    *time.Time // repair
	RootPieceID   storj.PieceID
	EncryptedSize int32 // size of the whole segment (not a piece)
	PlainOffset   int64 // verify
	PlainSize     int32 // verify
	Redundancy    storj.RedundancyScheme
	Pieces        Pieces
}

// Inline returns true if segment is inline.
func (s LoopSegmentEntry) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// IterateLoopStreams lists multiple streams segments.
func (db *DB) IterateLoopStreams(ctx context.Context, opts IterateLoopStreams, handleStream func(ctx context.Context, streamID uuid.UUID, next SegmentIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(opts.StreamIDs) == 0 {
		return ErrInvalidRequest.New("StreamIDs list is empty")
	}

	sort.Slice(opts.StreamIDs, func(i, k int) bool {
		return bytes.Compare(opts.StreamIDs[i][:], opts.StreamIDs[k][:]) < 0
	})

	// TODO do something like pgutil.UUIDArray()
	bytesIDs := make([][]byte, len(opts.StreamIDs))
	for i, streamID := range opts.StreamIDs {
		if streamID.IsZero() {
			return ErrInvalidRequest.New("StreamID missing: index %d", i)
		}
		id := streamID
		bytesIDs[i] = id[:]
	}

	var asOfSystemTime string
	if !opts.AsOfSystemTime.IsZero() && db.implementation == dbutil.Cockroach {
		asOfSystemTime = fmt.Sprintf(` AS OF SYSTEM TIME '%d' `, opts.AsOfSystemTime.UnixNano())
	}

	rows, err := db.db.Query(ctx, `
		SELECT
			stream_id, position,
			created_at, repaired_at,
			root_piece_id,
			encrypted_size,
			plain_offset, plain_size,
			redundancy,
			remote_alias_pieces
		FROM segments
		`+asOfSystemTime+`
		WHERE
		    -- this turns out to be a little bit faster than stream_id IN (SELECT unnest($1::BYTEA[]))
			stream_id = ANY ($1::BYTEA[])
		ORDER BY stream_id ASC, position ASC
	`, pgutil.ByteaArray(bytesIDs))
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

	var noMoreData bool
	var nextSegment *LoopSegmentEntry
	for _, streamID := range opts.StreamIDs {
		streamID := streamID
		var internalError error
		err := handleStream(ctx, streamID, func(output *LoopSegmentEntry) bool {
			if nextSegment != nil {
				if nextSegment.StreamID != streamID {
					return false
				}
				*output = *nextSegment
				nextSegment = nil
				return true
			}

			if noMoreData {
				return false
			}
			if !rows.Next() {
				noMoreData = true
				return false
			}

			var segment LoopSegmentEntry
			var aliasPieces AliasPieces
			err = rows.Scan(
				&segment.StreamID, &segment.Position,
				&segment.CreatedAt, &segment.RepairedAt,
				&segment.RootPieceID,
				&segment.EncryptedSize,
				&segment.PlainOffset, &segment.PlainSize,
				redundancyScheme{&segment.Redundancy},
				&aliasPieces,
			)
			if err != nil {
				internalError = Error.New("failed to scan segments: %w", err)
				return false
			}

			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				internalError = Error.New("failed to convert aliases to pieces: %w", err)
				return false
			}

			if segment.StreamID != streamID {
				nextSegment = &segment
				return false
			}

			*output = segment
			return true
		})
		if internalError != nil || err != nil {
			return Error.Wrap(errs.Combine(internalError, err))
		}
	}

	if !noMoreData {
		return Error.New("expected rows to be completely read")
	}

	return nil
}
