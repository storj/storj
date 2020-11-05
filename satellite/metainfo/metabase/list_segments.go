// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/uuid"
	"storj.io/storj/private/tagsql"
)

// ListSegments contains arguments necessary for listing stream segments.
type ListSegments struct {
	StreamID uuid.UUID
	Cursor   SegmentPosition
	Limit    int
}

// ListSegmentsResult result of listing segments.
type ListSegmentsResult struct {
	Segments []Segment
	More     bool
}

// ListSegments lists specified stream segments.
func (db *DB) ListSegments(ctx context.Context, opts ListSegments) (result ListSegmentsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return ListSegmentsResult{}, ErrInvalidRequest.New("StreamID missing")
	}

	// TODO verify this limit
	if opts.Limit > maxListLimit {
		return ListSegmentsResult{}, ErrInvalidRequest.New("Maximum listing limit is %d", maxListLimit)
	}
	if opts.Limit < 0 {
		return ListSegmentsResult{}, ErrInvalidRequest.New("Invalid limit: %d", opts.Limit)
	}

	if opts.Limit == 0 {
		opts.Limit = maxListLimit
	}

	err = withRows(db.db.Query(ctx, `
		SELECT
			position,
			root_piece_id, encrypted_key_nonce, encrypted_key,
			encrypted_size, plain_offset, plain_size,
			redundancy,
			inline_data, remote_pieces
		FROM segments
		WHERE
			stream_id = $1 AND
			($2 = 0 OR position > $2)
		ORDER BY position ASC
		LIMIT $3
	`, opts.StreamID, opts.Cursor, opts.Limit+1))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment Segment
			err = rows.Scan(
				&segment.Position,
				&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
				&segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize,
				redundancyScheme{&segment.Redundancy},
				&segment.InlineData, &segment.Pieces,
			)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			segment.StreamID = opts.StreamID
			result.Segments = append(result.Segments, segment)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListSegmentsResult{}, nil
		}
		return ListSegmentsResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	if len(result.Segments) > opts.Limit {
		result.More = true
		result.Segments = result.Segments[:len(result.Segments)-1]
	}

	return result, nil
}
