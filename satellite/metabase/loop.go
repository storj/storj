// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"math"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// MaxLoopIteratorBatchSize is the largest BatchSize IterateLoopSegments accepts.
// Callers taking their batch size from configuration should cap it here, because
// a larger one is rejected rather than clamped.
const MaxLoopIteratorBatchSize = 50000

const loopIteratorBatchSizeLimit = intLimitRange(MaxLoopIteratorBatchSize)

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
	// BatchSize is how many segments are read per query. Zero means
	// MaxLoopIteratorBatchSize; anything above it is rejected rather than
	// clamped, so a caller grouping entries by the size it asked for keeps
	// pace with the pages the iterator hands out.
	BatchSize          int
	StartStreamID      uuid.UUID
	EndStreamID        uuid.UUID
	AsOfSystemInterval time.Duration
	// ReadTimestamp makes all queries read a consistent snapshot of the
	// database at the given timestamp. It takes precedence over
	// AsOfSystemInterval and only backends providing fixed-timestamp reads
	// (TiDB, CockroachDB) can serve it; the others fail the request.
	ReadTimestamp time.Time
	// AllowLiveReads lets a metabase whose backends cannot serve ReadTimestamp
	// read live instead of failing: DB.IterateLoopSegments drops the timestamp
	// when no adapter can serve it, and refuses a mixed metabase, where the
	// adapters that can would read a snapshot the others do not. Meant for
	// testing and restored backups only, as the scan then reads no snapshot.
	AllowLiveReads bool
}

// Verify verifies segments request fields.
func (opts *IterateLoopSegments) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	// Refuse to silently page at a smaller size than asked for: callers group the
	// entries they receive by the size they requested, and the iterator's buffers
	// are only safe to reuse once the page they belong to has been handed over.
	if opts.BatchSize > MaxLoopIteratorBatchSize {
		return ErrInvalidRequest.New("BatchSize is too large, maximum is %d", MaxLoopIteratorBatchSize)
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

	if opts.AllowLiveReads && !opts.ReadTimestamp.IsZero() {
		// Postgres has no AS OF SYSTEM TIME, so a fixed read timestamp is not
		// something it can be asked for. The caller chose to let it read live
		// rather than fail the scan, which is only a snapshot when no backend
		// could have served the timestamp: on a mixed metabase the others
		// would read a snapshot it does not, so that is refused here, where
		// the timestamp would be dropped, rather than left to the callers.
		serving := 0
		for _, a := range db.adapters {
			if a.Implementation().AsOfSystemTime(opts.ReadTimestamp) != "" {
				serving++
			}
		}
		switch {
		case serving == 0:
			opts.ReadTimestamp = time.Time{}
		case serving < len(db.adapters):
			return ErrInvalidRequest.New("live reads on a mixed metabase would read a partial snapshot")
		}
	}

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

// sqlRebinder rewrites `?` placeholders to a backend-specific form (`$N` for
// Postgres/Cockroach). TiDB-style backends don't satisfy this interface, which
// is exactly how the opportunistic dispatch picks the right shape.
type sqlRebinder interface {
	Rebind(string) string
}

// rebindIfNeeded applies the Rebind method when the underlying DB exposes one,
// otherwise returns sql unchanged.
func rebindIfNeeded(db tagsql.DB, sql string) string {
	if r, ok := db.(sqlRebinder); ok {
		return r.Rebind(sql)
	}
	return sql
}

// IterateLoopSegments implements Adapter.
func (p *PostgresAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	return tagsqlIterateLoopSegments(ctx, p, aliasCache, opts, fn)
}

// IterateLoopSegments implements Adapter.
func (c *CockroachAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	return tagsqlIterateLoopSegments(ctx, c, aliasCache, opts, fn)
}

// IterateLoopSegments implements Adapter.
func (t *TiDBAdapter) IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	return tagsqlIterateLoopSegments(ctx, t, aliasCache, opts, fn)
}

func tagsqlIterateLoopSegments(ctx context.Context, db tagsqlAdapter, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !opts.ReadTimestamp.IsZero() && db.Implementation().AsOfSystemTime(opts.ReadTimestamp) == "" {
		// Refuse to silently fall back to live reads: callers setting
		// ReadTimestamp depend on a consistent snapshot for correctness
		// (e.g. garbage collection bloom filters).
		return ErrInvalidRequest.New("ReadTimestamp is not supported on %v", db.Implementation())
	}

	it := &tagsqlLoopSegmentIterator{
		db:         db,
		aliasCache: aliasCache,

		asOfSystemInterval: opts.AsOfSystemInterval,
		readTimestamp:      opts.ReadTimestamp,
		batchSize:          opts.BatchSize,
		batchPieces:        make([]Pieces, opts.BatchSize),
		batchAliasPieces:   make([]AliasPieces, opts.BatchSize),

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
	// batchPieces and batchAliasPieces are reused between result pages to reduce
	// memory consumption. They are indexed by the position within the page so that
	// every entry of a batch keeps its own storage: consumers are handed the whole
	// batch at once, so sharing one buffer would leave every entry pointing at the
	// last scanned row.
	//
	// Reuse between pages is only safe while a consumer's batch never spans two
	// pages. Verify rejects a BatchSize above the limit rather than clamping it,
	// so a consumer grouping by the size it requested pages in lockstep with us.
	batchPieces      []Pieces
	batchAliasPieces []AliasPieces

	asOfSystemInterval time.Duration
	readTimestamp      time.Time

	curIndex int
	curRows  tagsql.Rows
	cursor   loopSegmentIteratorCursor

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

// Next returns true if there was another item and copy it in item.
func (it *tagsqlLoopSegmentIterator) Next(ctx context.Context, item *LoopSegmentEntry) bool {
	if err := ctx.Err(); err != nil {
		it.failErr = errs.Combine(it.failErr, err)
		return false
	}
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

	asOf := impl.AsOfSystemInterval(it.asOfSystemInterval)
	if !it.readTimestamp.IsZero() {
		asOf = impl.AsOfSystemTime(it.readTimestamp)
	}

	sql := rebindIfNeeded(db, `
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
		`+asOf+`
		WHERE
			(stream_id, position) > (?, ?) AND stream_id <= ?
		ORDER BY stream_id ASC, position ASC
		LIMIT ?`)
	return db.QueryContext(ctx, sql,
		it.cursor.StartStreamID, it.cursor.StartPosition.Encode(),
		it.cursor.EndStreamID, it.batchSize,
	)
}

// scanItem scans doNextQuery results into LoopSegmentEntry.
func (it *tagsqlLoopSegmentIterator) scanItem(ctx context.Context, item *LoopSegmentEntry) error {
	// AliasPieces.SetBytes reuses the backing array when it has capacity, so scan into
	// this position's own buffer rather than whatever the previous row left behind.
	item.AliasPieces = it.batchAliasPieces[it.curIndex]

	err := it.curRows.Scan(
		&item.StreamID, &item.Position,
		&item.CreatedAt, &item.ExpiresAt, &item.RepairedAt,
		&item.RootPieceID,
		&item.EncryptedSize,
		&item.PlainOffset, &item.PlainSize,
		&item.Redundancy,
		&item.AliasPieces,
		&item.Placement,
	)
	if err != nil {
		return Error.New("failed to scan segments: %w", err)
	}
	// keep the grown buffer so the next page over this position can reuse it
	it.batchAliasPieces[it.curIndex] = item.AliasPieces

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
