// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// ListVerifyLimit is the maximum number of items the client can request for listing.
const ListVerifyLimit = intLimitRange(100000)

// ListVerifySegments contains arguments necessary for listing stream segments.
type ListVerifySegments struct {
	CursorStreamID uuid.UUID
	CursorPosition SegmentPosition
	Limit          int

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
}

// ListVerifySegmentsResult is the result of ListVerifySegments.
type ListVerifySegmentsResult struct {
	Segments []VerifySegment
}

// VerifySegment result of listing segments for verifying remote segments.
type VerifySegment struct {
	StreamID uuid.UUID
	Position SegmentPosition

	CreatedAt  time.Time
	RepairedAt *time.Time

	RootPieceID storj.PieceID
	Redundancy  storj.RedundancyScheme

	AliasPieces AliasPieces
}

// ListVerifySegments lists specified stream segments.
func (db *DB) ListVerifySegments(ctx context.Context, opts ListVerifySegments) (result ListVerifySegmentsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.Limit <= 0 {
		return ListVerifySegmentsResult{}, ErrInvalidRequest.New("Invalid limit: %d", opts.Limit)
	}
	ListVerifyLimit.Ensure(&opts.Limit)
	result.Segments = make([]VerifySegment, 0, opts.Limit)

	err = withRows(db.db.QueryContext(ctx, `
		SELECT
			stream_id, position,
			created_at, repaired_at,
			root_piece_id, redundancy,
			remote_alias_pieces
		FROM segments
		`+db.asOfTime(opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
		WHERE
			(stream_id, position) > ($1, $2) AND
			inline_data IS NULL AND
			remote_alias_pieces IS NOT NULL
		ORDER BY stream_id ASC, position ASC
		LIMIT $3
	`, opts.CursorStreamID, opts.CursorPosition, opts.Limit))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var seg VerifySegment
			err := rows.Scan(
				&seg.StreamID,
				&seg.Position,

				&seg.CreatedAt,
				&seg.RepairedAt,

				&seg.RootPieceID,
				redundancyScheme{&seg.Redundancy},
				&seg.AliasPieces,
			)
			if err != nil {
				return Error.Wrap(err)
			}

			result.Segments = append(result.Segments, seg)
		}
		return nil
	})

	return result, Error.Wrap(err)
}
