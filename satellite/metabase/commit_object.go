// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"math"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

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
