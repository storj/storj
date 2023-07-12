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
	"storj.io/private/tagsql"
)

const loopIteratorBatchSizeLimit = intLimitRange(5000)

// IterateLoopObjects contains arguments necessary for listing objects in metabase.
type IterateLoopObjects struct {
	BatchSize int

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
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
	TotalEncryptedSize    int64        // tally
	EncryptedMetadataSize int          // tally
}

// Expired checks if object is expired relative to now.
func (o LoopObjectEntry) Expired(now time.Time) bool {
	return o.ExpiresAt != nil && o.ExpiresAt.Before(now)
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

		curIndex:           0,
		cursor:             loopIterateCursor{},
		asOfSystemTime:     opts.AsOfSystemTime,
		asOfSystemInterval: opts.AsOfSystemInterval,
	}

	loopIteratorBatchSizeLimit.Ensure(&it.batchSize)

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

// loopIterator enables iteration of all objects in metabase.
type loopIterator struct {
	db *DB

	batchSize          int
	asOfSystemTime     time.Time
	asOfSystemInterval time.Duration

	curIndex int
	curRows  tagsql.Rows
	cursor   loopIterateCursor

	// failErr is set when either scan or next query fails during iteration.
	failErr error
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
			it.failErr = errs.Combine(it.failErr, err)
			return false
		}

		if closeErr := it.curRows.Close(); closeErr != nil {
			it.failErr = errs.Combine(it.failErr, closeErr, rows.Close())
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
		it.failErr = errs.Combine(it.failErr, err)
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

	return it.db.db.QueryContext(ctx, `
		SELECT
			project_id, bucket_name,
			object_key, stream_id, version,
			status,
			created_at, expires_at,
			segment_count, total_encrypted_size,
			LENGTH(COALESCE(encrypted_metadata,''))
		FROM objects
		`+it.db.asOfTime(it.asOfSystemTime, it.asOfSystemInterval)+`
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
		&item.SegmentCount, &item.TotalEncryptedSize,
		&item.EncryptedMetadataSize,
	)
}

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
	BatchSize          int
	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
	StartStreamID      uuid.UUID
	EndStreamID        uuid.UUID
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

	it := &loopSegmentIterator{
		db: db,

		asOfSystemTime:     opts.AsOfSystemTime,
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

// loopSegmentIterator enables iteration of all segments in metabase.
type loopSegmentIterator struct {
	db *DB

	batchSize int
	// batchPieces are reused between result pages to reduce memory consumption
	batchPieces []Pieces

	asOfSystemTime     time.Time
	asOfSystemInterval time.Duration

	curIndex int
	curRows  tagsql.Rows
	cursor   loopSegmentIteratorCursor

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

type loopSegmentIteratorCursor struct {
	StartStreamID uuid.UUID
	StartPosition SegmentPosition
	EndStreamID   uuid.UUID
}

// Next returns true if there was another item and copy it in item.
func (it *loopSegmentIterator) Next(ctx context.Context, item *LoopSegmentEntry) bool {
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

func (it *loopSegmentIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.QueryContext(ctx, `
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
		`+it.db.asOfTime(it.asOfSystemTime, it.asOfSystemInterval)+`
		WHERE
			(stream_id, position) > ($1, $2) AND stream_id <= $4
		ORDER BY (stream_id, position) ASC
		LIMIT $3
		`, it.cursor.StartStreamID, it.cursor.StartPosition.Encode(),
		it.batchSize, it.cursor.EndStreamID,
	)
}

// scanItem scans doNextQuery results into LoopSegmentEntry.
func (it *loopSegmentIterator) scanItem(ctx context.Context, item *LoopSegmentEntry) error {
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

	item.Pieces, err = it.db.aliasCache.convertAliasesToPieces(ctx, item.AliasPieces, it.batchPieces[it.curIndex])
	if err != nil {
		return Error.New("failed to convert aliases to pieces: %w", err)
	}

	return nil
}

// BucketTally contains information about aggregate data stored in a bucket.
type BucketTally struct {
	BucketLocation

	ObjectCount        int64
	PendingObjectCount int64

	TotalSegments int64
	TotalBytes    int64

	MetadataSize int64
}

// CollectBucketTallies contains arguments necessary for looping through objects in metabase.
type CollectBucketTallies struct {
	From               BucketLocation
	To                 BucketLocation
	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
	Now                time.Time
}

// Verify verifies CollectBucketTallies request fields.
func (opts *CollectBucketTallies) Verify() error {
	if opts.To.ProjectID.Less(opts.From.ProjectID) {
		return ErrInvalidRequest.New("project ID To is before project ID From")
	}
	if opts.To.ProjectID == opts.From.ProjectID && opts.To.BucketName < opts.From.BucketName {
		return ErrInvalidRequest.New("bucket name To is before bucket name From")
	}
	return nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (db *DB) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return []BucketTally{}, err
	}

	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	err = withRows(db.db.QueryContext(ctx, `
			SELECT
				project_id, bucket_name,
				SUM(total_encrypted_size), SUM(segment_count), COALESCE(SUM(length(encrypted_metadata)), 0),
				count(*), count(*) FILTER (WHERE status = 1)
			FROM objects
			`+db.asOfTime(opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
			WHERE (project_id, bucket_name) BETWEEN ($1, $2) AND ($3, $4) AND
			(expires_at IS NULL OR expires_at > $5)
			GROUP BY (project_id, bucket_name)
			ORDER BY (project_id, bucket_name) ASC
		`, opts.From.ProjectID, []byte(opts.From.BucketName), opts.To.ProjectID, []byte(opts.To.BucketName), opts.Now))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var bucketTally BucketTally

			if err = rows.Scan(
				&bucketTally.ProjectID, &bucketTally.BucketName,
				&bucketTally.TotalBytes, &bucketTally.TotalSegments,
				&bucketTally.MetadataSize, &bucketTally.ObjectCount,
				&bucketTally.PendingObjectCount,
			); err != nil {
				return Error.New("unable to query bucket tally: %w", err)
			}

			result = append(result, bucketTally)
		}

		return nil
	})
	if err != nil {
		return []BucketTally{}, err
	}

	return result, nil
}
