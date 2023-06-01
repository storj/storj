// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/tagsql"
)

// ListSegments contains arguments necessary for listing stream segments.
type ListSegments struct {
	StreamID uuid.UUID
	Cursor   SegmentPosition
	Limit    int

	Range *StreamRange

	// This causes ListSegments to update the first Segment in the response
	// with the ancestor info if it exists and server side copy is enabled.
	UpdateFirstWithAncestor bool
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

	if opts.Limit < 0 {
		return ListSegmentsResult{}, ErrInvalidRequest.New("Invalid limit: %d", opts.Limit)
	}

	ListLimit.Ensure(&opts.Limit)

	if opts.Range != nil {
		if opts.Range.PlainStart > opts.Range.PlainLimit {
			return ListSegmentsResult{}, ErrInvalidRequest.New("invalid range: %d:%d", opts.Range.PlainStart, opts.Range.PlainLimit)
		}
	}

	var rows tagsql.Rows
	var rowsErr error
	if opts.Range == nil {
		rows, rowsErr = db.db.QueryContext(ctx, `
			SELECT
				position, created_at, expires_at, root_piece_id,
				encrypted_key_nonce, encrypted_key, encrypted_size,
				plain_offset, plain_size, encrypted_etag, redundancy,
				inline_data, remote_alias_pieces
			FROM segments
			WHERE
				stream_id = $1 AND
				($2 = 0::INT8 OR position > $2)
			ORDER BY stream_id, position ASC
			LIMIT $3
		`, opts.StreamID, opts.Cursor, opts.Limit+1)
	} else {
		rows, rowsErr = db.db.QueryContext(ctx, `
			SELECT
				position, created_at, expires_at, root_piece_id,
				encrypted_key_nonce, encrypted_key, encrypted_size,
				plain_offset, plain_size, encrypted_etag, redundancy,
				inline_data, remote_alias_pieces
			FROM segments
			WHERE
				stream_id = $1 AND
				($2 = 0::INT8 OR position > $2) AND
				$4 < plain_offset + plain_size AND plain_offset < $5
			ORDER BY stream_id, position ASC
			LIMIT $3
		`, opts.StreamID, opts.Cursor, opts.Limit+1, opts.Range.PlainStart, opts.Range.PlainLimit)
	}

	err = withRows(rows, rowsErr)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment Segment
			var aliasPieces AliasPieces
			err = rows.Scan(
				&segment.Position,
				&segment.CreatedAt, &segment.ExpiresAt,
				&segment.RootPieceID, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
				&segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize,
				&segment.EncryptedETag,
				redundancyScheme{&segment.Redundancy},
				&segment.InlineData, &aliasPieces,
			)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			segment.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
			if err != nil {
				return Error.New("failed to convert aliases to pieces: %w", err)
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

	if db.config.ServerSideCopy {
		copies := make([]Segment, 0, len(result.Segments))
		copiesPositions := make([]int64, 0, len(result.Segments))
		for _, segment := range result.Segments {
			if segment.PiecesInAncestorSegment() {
				copies = append(copies, segment)
				copiesPositions = append(copiesPositions, int64(segment.Position.Encode()))
			}
		}

		if len(copies) > 0 {
			index := 0
			err = withRows(db.db.QueryContext(ctx, `
					SELECT
						root_piece_id,
						remote_alias_pieces
					FROM segments as segments
					LEFT JOIN segment_copies as copies
					ON copies.ancestor_stream_id = segments.stream_id
					WHERE
						copies.stream_id = $1 AND segments.position IN (SELECT position FROM UNNEST($2::INT8[]) as position)
					ORDER BY segments.stream_id, segments.position ASC
				`, opts.StreamID, pgutil.Int8Array(copiesPositions)))(func(rows tagsql.Rows) error {

				for rows.Next() {
					var aliasPieces AliasPieces
					err = rows.Scan(
						&copies[index].RootPieceID,
						&aliasPieces,
					)
					if err != nil {
						return Error.New("failed to scan segments: %w", err)
					}

					copies[index].Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, aliasPieces)
					if err != nil {
						return Error.New("failed to convert aliases to pieces: %w", err)
					}
					index++
				}
				return nil
			})
			if err != nil {
				return ListSegmentsResult{}, Error.New("unable to fetch object segments: %w", err)
			}

			if index != len(copies) {
				return ListSegmentsResult{}, Error.New("number of ancestor segments is different than copies: want %d got %d",
					len(copies), index)
			}
		}

		// we have to update the first segment because DownloadObject uses this call
		// and we only need to do the first segment because it only uses the extra
		// information for the first segment.
		if len(result.Segments) > 0 && opts.UpdateFirstWithAncestor {
			err = db.updateWithAncestorSegment(ctx, &result.Segments[0])
			if err != nil {
				return ListSegmentsResult{}, err
			}
		}
	}

	if len(result.Segments) > opts.Limit {
		result.More = true
		result.Segments = result.Segments[:len(result.Segments)-1]
	}

	return result, nil
}

// ListStreamPositions contains arguments necessary for listing stream segments.
type ListStreamPositions struct {
	StreamID uuid.UUID
	Cursor   SegmentPosition
	Limit    int

	Range *StreamRange
}

// StreamRange allows to limit stream positions based on the plain offsets.
type StreamRange struct {
	PlainStart int64
	PlainLimit int64 // limit is exclusive
}

// ListStreamPositionsResult result of listing segments.
type ListStreamPositionsResult struct {
	Segments []SegmentPositionInfo
	More     bool
}

// SegmentPositionInfo contains information for segment position.
type SegmentPositionInfo struct {
	Position SegmentPosition
	// PlainSize is 0 for a migrated object.
	PlainSize int32
	// PlainOffset is 0 for a migrated object.
	PlainOffset       int64
	CreatedAt         *time.Time // TODO: make it non-nilable after we migrate all existing segments to have creation time
	EncryptedETag     []byte
	EncryptedKeyNonce []byte
	EncryptedKey      []byte
}

// ListStreamPositions lists specified stream segment positions.
func (db *DB) ListStreamPositions(ctx context.Context, opts ListStreamPositions) (result ListStreamPositionsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return ListStreamPositionsResult{}, ErrInvalidRequest.New("StreamID missing")
	}

	if opts.Limit < 0 {
		return ListStreamPositionsResult{}, ErrInvalidRequest.New("Invalid limit: %d", opts.Limit)
	}

	ListLimit.Ensure(&opts.Limit)

	if opts.Range != nil {
		if opts.Range.PlainStart > opts.Range.PlainLimit {
			return ListStreamPositionsResult{}, ErrInvalidRequest.New("invalid range: %d:%d", opts.Range.PlainStart, opts.Range.PlainLimit)
		}
	}

	var rows tagsql.Rows
	var rowsErr error
	if opts.Range == nil {
		rows, rowsErr = db.db.QueryContext(ctx, `
			SELECT
				position, plain_size, plain_offset, created_at,
				encrypted_etag, encrypted_key_nonce, encrypted_key
			FROM segments
			WHERE
				stream_id = $1 AND
				($2 = 0::INT8 OR position > $2)
			ORDER BY position ASC
			LIMIT $3
		`, opts.StreamID, opts.Cursor, opts.Limit+1)
	} else {
		rows, rowsErr = db.db.QueryContext(ctx, `
			SELECT
				position, plain_size, plain_offset, created_at,
				encrypted_etag, encrypted_key_nonce, encrypted_key
			FROM segments
			WHERE
				stream_id = $1 AND
				($2 = 0::INT8 OR position > $2) AND
				$4 < plain_offset + plain_size AND plain_offset < $5
			ORDER BY position ASC
			LIMIT $3
		`, opts.StreamID, opts.Cursor, opts.Limit+1, opts.Range.PlainStart, opts.Range.PlainLimit)
	}

	err = withRows(rows, rowsErr)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var segment SegmentPositionInfo
			err = rows.Scan(
				&segment.Position, &segment.PlainSize, &segment.PlainOffset, &segment.CreatedAt,
				&segment.EncryptedETag, &segment.EncryptedKeyNonce, &segment.EncryptedKey,
			)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}
			result.Segments = append(result.Segments, segment)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListStreamPositionsResult{}, nil
		}
		return ListStreamPositionsResult{}, Error.New("unable to fetch object segments: %w", err)
	}

	if len(result.Segments) > opts.Limit {
		result.More = true
		result.Segments = result.Segments[:len(result.Segments)-1]
	}

	return result, nil
}
