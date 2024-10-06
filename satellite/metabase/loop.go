// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

const loopIteratorBatchSizeLimit = intLimitRange(5000)

// SegmentIterator returns the next segment.
type SegmentIterator func(ctx context.Context, segment *LoopSegmentEntry) bool

// LoopSegmentEntry contains information about segment metadata needed by metainfo loop.
type LoopSegmentEntry struct {
	StreamID      uuid.UUID
	Position      SegmentPosition
	CreatedAt     time.Time // non-nillable
	ExpiresAt     *time.Time
	RepairedAt    *time.Time // repair
	RootPieceID   storj.PieceID
	EncryptedSize int32 // size of the whole segment (not a piece)
	PlainOffset   int64 // verify
	PlainSize     int32 // verify
	AliasPieces   AliasPieces
	Redundancy    storj.RedundancyScheme
	Pieces        Pieces
	Placement     storj.PlacementConstraint
	Source        string
}

// Inline returns true if segment is inline.
func (s LoopSegmentEntry) Inline() bool {
	return s.Redundancy.IsZero() && len(s.Pieces) == 0
}

// LoopSegmentsIterator iterates over a sequence of LoopSegmentEntry items.
type LoopSegmentsIterator interface {
	Next(ctx context.Context, item *LoopSegmentEntry) bool
}

// IterateLoopSegments contains arguments necessary for listing segments in metabase.
type IterateLoopSegments struct {
	BatchSize            int
	StartStreamID        uuid.UUID
	EndStreamID          uuid.UUID
	AsOfSystemInterval   time.Duration
	SpannerReadTimestamp time.Time
}

// Verify verifies segments request fields.
func (opts *IterateLoopSegments) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	if !opts.EndStreamID.IsZero() {
		if opts.EndStreamID.Less(opts.StartStreamID) {
			return ErrInvalidRequest.New("EndStreamID is smaller than StartStreamID")
		}
		if opts.StartStreamID == opts.EndStreamID {
			return ErrInvalidRequest.New("StartStreamID and EndStreamID must be different")
		}
	}
	return nil
}

// IterateLoopSegments iterates through all segments in metabase.
func (db *DB) IterateLoopSegments(ctx context.Context, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	loopIteratorBatchSizeLimit.Ensure(&opts.BatchSize)

	for _, a := range db.adapters {
		err := a.IterateLoopSegments(ctx, db.aliasCache, opts, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

type tagsqlAdapter interface {
	Name() string
	UnderlyingDB() tagsql.DB
	Implementation() dbutil.Implementation
}

// IterateLoopSegments implements Adapter.
func (p *PostgresAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	return tagsqlIterateLoopSegments(ctx, p, aliasCache, opts, fn)
}

// IterateLoopSegments implements Adapter.
func (c *CockroachAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	return tagsqlIterateLoopSegments(ctx, c, aliasCache, opts, fn)
}

func tagsqlIterateLoopSegments(ctx context.Context, db tagsqlAdapter, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	it := &tagsqlLoopSegmentIterator{
		db:         db,
		aliasCache: aliasCache,

		asOfSystemInterval: opts.AsOfSystemInterval,
		batchSize:          opts.BatchSize,
		batchPieces:        make([]Pieces, opts.BatchSize),

		curIndex: 0,
		cursor: loopSegmentIteratorCursor{
			StartStreamID: opts.StartStreamID,
			EndStreamID:   opts.EndStreamID,
		},
	}

	if !opts.StartStreamID.IsZero() {
		// uses MaxInt32 instead of MaxUint32 because position is an int8 in db.
		it.cursor.StartPosition = SegmentPosition{math.MaxInt32, math.MaxInt32}
	}
	if it.cursor.EndStreamID.IsZero() {
		it.cursor.EndStreamID = uuid.Max()
	}

	it.curRows, err = it.doNextQuery(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if rowsErr := it.curRows.Err(); rowsErr != nil {
			err = errs.Combine(err, rowsErr)
		}
		err = errs.Combine(err, it.failErr, it.curRows.Close())
	}()

	return fn(ctx, it)
}

type loopSegmentIteratorCursor struct {
	StartStreamID uuid.UUID
	StartPosition SegmentPosition
	EndStreamID   uuid.UUID
}

// tagsqlLoopSegmentIterator enables iteration of all segments in metabase.
type tagsqlLoopSegmentIterator struct {
	db         tagsqlAdapter
	aliasCache *NodeAliasCache

	batchSize int
	// batchPieces are reused between result pages to reduce memory consumption
	batchPieces []Pieces

	asOfSystemInterval time.Duration

	curIndex int
	curRows  tagsql.Rows
	cursor   loopSegmentIteratorCursor

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

// Next returns true if there was another item and copy it in item.
func (it *tagsqlLoopSegmentIterator) Next(ctx context.Context, item *LoopSegmentEntry) bool {
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
			it.failErr = errs.Combine(it.failErr, err)
			return false
		}

		if failErr := it.curRows.Close(); failErr != nil {
			it.failErr = errs.Combine(it.failErr, failErr, rows.Close())
			return false
		}

		it.curRows = rows
		it.curIndex = 0
		if !it.curRows.Next() {
			return false
		}
	}

	err := it.scanItem(ctx, item)
	if err != nil {
		it.failErr = errs.Combine(it.failErr, err)
		return false
	}

	it.curIndex++
	it.cursor.StartStreamID = item.StreamID
	it.cursor.StartPosition = item.Position

	return true
}

func (it *tagsqlLoopSegmentIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	db := it.db.UnderlyingDB()
	impl := it.db.Implementation()
	return db.QueryContext(ctx, `
		SELECT
			stream_id, position,
			created_at, expires_at, repaired_at,
			root_piece_id,
			encrypted_size,
			plain_offset, plain_size,
			redundancy,
			remote_alias_pieces,
			placement
		FROM segments
		`+impl.AsOfSystemInterval(it.asOfSystemInterval)+`
		WHERE
			(stream_id, position) > ($1, $2) AND stream_id <= $4
		ORDER BY (stream_id, position) ASC
		LIMIT $3
		`, it.cursor.StartStreamID, it.cursor.StartPosition.Encode(),
		it.batchSize, it.cursor.EndStreamID,
	)
}

// scanItem scans doNextQuery results into LoopSegmentEntry.
func (it *tagsqlLoopSegmentIterator) scanItem(ctx context.Context, item *LoopSegmentEntry) error {
	err := it.curRows.Scan(
		&item.StreamID, &item.Position,
		&item.CreatedAt, &item.ExpiresAt, &item.RepairedAt,
		&item.RootPieceID,
		&item.EncryptedSize,
		&item.PlainOffset, &item.PlainSize,
		redundancyScheme{&item.Redundancy},
		&item.AliasPieces,
		&item.Placement,
	)
	if err != nil {
		return Error.New("failed to scan segments: %w", err)
	}

	// allocate new Pieces only if existing have not enough capacity
	if cap(it.batchPieces[it.curIndex]) < len(item.AliasPieces) {
		it.batchPieces[it.curIndex] = make(Pieces, len(item.AliasPieces))
	} else {
		it.batchPieces[it.curIndex] = it.batchPieces[it.curIndex][:len(item.AliasPieces)]
	}

	item.Pieces, err = it.aliasCache.convertAliasesToPieces(ctx, item.AliasPieces, it.batchPieces[it.curIndex])
	if err != nil {
		return Error.New("failed to convert aliases to pieces: %w", err)
	}
	item.Source = it.db.Name()

	return nil
}

type spannerLoopSegmentIterator struct {
	db *SpannerAdapter

	batchSize int
	// TODO(spanner) would be nice to have it at some point
	// batchPieces are reused between result pages to reduce memory consumption
	// batchPieces []Pieces

	readTimestamp time.Time

	curIndex int
	curRows  *spanner.RowIterator
	curRow   *spanner.Row
	cursor   loopSegmentIteratorCursor

	// failErr is set when either scan or next query fails during iteration.
	failErr    error
	aliasCache *NodeAliasCache
}

// Next returns true if there was another item and copy it in item.
func (it *spannerLoopSegmentIterator) Next(ctx context.Context, item *LoopSegmentEntry) bool {
	var err error
	it.curRow, err = it.curRows.Next()
	next := !errors.Is(err, iterator.Done)
	if err != nil && next {
		it.failErr = errs.Combine(it.failErr, err)
		return false
	}

	if !next {
		if it.curIndex < it.batchSize {
			return false
		}

		rows := it.doNextQuery(ctx)

		it.curRows.Stop()

		it.curRows = rows
		it.curIndex = 0

		it.curRow, err = it.curRows.Next()
		if err != nil {
			if !errors.Is(err, iterator.Done) {
				it.failErr = errs.Combine(it.failErr, err)
			}
			return false
		}
	}

	err = it.scanItem(ctx, item)
	if err != nil {
		it.failErr = errs.Combine(it.failErr, err)
		return false
	}

	it.curIndex++
	it.cursor.StartStreamID = item.StreamID
	it.cursor.StartPosition = item.Position
	return true
}

func (it *spannerLoopSegmentIterator) doNextQuery(ctx context.Context) (_ *spanner.RowIterator) {
	stmt := spanner.Statement{
		SQL: `
			SELECT
				stream_id, position,
				created_at, expires_at, repaired_at,
				root_piece_id,
				encrypted_size,
				plain_offset, plain_size,
				redundancy,
				remote_alias_pieces,
				placement
			FROM segments
			WHERE
				(stream_id > @streamid OR (stream_id = @streamid AND position > @position)) AND stream_id <= @endstreamid
			ORDER BY stream_id ASC, position ASC
			LIMIT @batchsize
		`,
		Params: map[string]interface{}{
			"streamid":    it.cursor.StartStreamID.Bytes(),
			"position":    int64(it.cursor.StartPosition.Encode()),
			"endstreamid": it.cursor.EndStreamID.Bytes(),
			"batchsize":   it.batchSize,
		}}

	if it.readTimestamp.IsZero() {
		return it.db.client.Single().Query(ctx, stmt)
	}
	return it.db.client.Single().WithTimestampBound(spanner.ReadTimestamp(it.readTimestamp)).Query(ctx, stmt)
}

// IterateLoopSegments implements Adapter.
func (s *SpannerAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	it := &spannerLoopSegmentIterator{
		db:         s,
		aliasCache: aliasCache,

		readTimestamp: opts.SpannerReadTimestamp,

		batchSize: opts.BatchSize,

		curIndex: 0,
		cursor: loopSegmentIteratorCursor{
			StartStreamID: opts.StartStreamID,
			EndStreamID:   opts.EndStreamID,
		},
	}

	if !opts.StartStreamID.IsZero() {
		// uses MaxInt32 instead of MaxUint32 because position is an int8 in db.
		it.cursor.StartPosition = SegmentPosition{math.MaxInt32, math.MaxInt32}
	}
	if it.cursor.EndStreamID.IsZero() {
		it.cursor.EndStreamID = uuid.Max()
	}

	it.curRows = it.doNextQuery(ctx)

	defer func() {
		it.curRows.Stop()
	}()

	return fn(ctx, it)
}

func (it *spannerLoopSegmentIterator) scanItem(ctx context.Context, item *LoopSegmentEntry) (err error) {
	var position int64
	var createdAt time.Time
	var repairedAt, expiresAt spanner.NullTime
	var encryptedSize, plainOffset, plainSize, placement int64

	if it.db.Implementation() == dbutil.Spanner {
		spannerPlacement := sql.NullInt64{}
		if err := it.curRow.Columns(&item.StreamID, &position,
			&createdAt, &expiresAt, &repairedAt,
			&item.RootPieceID,
			&encryptedSize, &plainOffset, &plainSize,
			redundancyScheme{&item.Redundancy},
			&item.AliasPieces,
			&spannerPlacement,
		); err != nil {
			return Error.New("failed to scan segment: %w", err)
		}
		if spannerPlacement.Valid {
			placement = spannerPlacement.Int64
		}
	} else {
		if err := it.curRow.Columns(&item.StreamID, &position,
			&createdAt, &expiresAt, &repairedAt,
			&item.RootPieceID,
			&encryptedSize, &plainOffset, &plainSize,
			redundancyScheme{&item.Redundancy},
			&item.AliasPieces,
			&placement,
		); err != nil {
			return Error.New("failed to scan segment: %w", err)
		}

	}

	item.Position = SegmentPositionFromEncoded(uint64(position))
	item.CreatedAt = createdAt
	if repairedAt.Valid {
		item.RepairedAt = &repairedAt.Time
	} else {
		item.RepairedAt = nil
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	} else {
		item.ExpiresAt = nil
	}
	item.EncryptedSize = int32(encryptedSize)
	item.PlainOffset = plainOffset
	item.PlainSize = int32(plainSize)

	item.Placement = storj.PlacementConstraint(placement)
	item.Pieces, err = it.aliasCache.ConvertAliasesToPieces(ctx, item.AliasPieces)
	if err != nil {
		return Error.New("failed to scan segment: %w", err)
	}
	item.Source = it.db.Name()

	return nil
}
