// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// CheckSegmentPiecesAlteration checks if the segment with streamID, and position is present in the
// DB and its pieces match pieces.
//
// It returns true if the pieces don't match, otherwise false.
//
// It returns an error of class `ErrSegmentNotFound` if the segment doesn't exist in the DB or any
// other type if there is another kind of error.
func (db *DB) CheckSegmentPiecesAlteration(
	ctx context.Context, streamID uuid.UUID, position SegmentPosition, pieces Pieces,
) (altered bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if streamID.IsZero() {
		return false, ErrInvalidRequest.New("StreamID missing")
	}

	if len(pieces) == 0 {
		return false, ErrInvalidRequest.New("pieces missing")
	}

	// Convert pieces to alias pieces as they are stored in the database
	aliasPieces, err := db.aliasCache.EnsurePiecesToAliases(ctx, pieces)
	if err != nil {
		return false, Error.New("unable to convert pieces to aliases: %w", err)
	}

	// Check all adapters until a match is found
	found := false
	for _, adapter := range db.adapters {
		altered, err = adapter.CheckSegmentPiecesAlteration(ctx, streamID, position, aliasPieces)
		if err != nil {
			if ErrSegmentNotFound.Has(err) {
				continue
			}
			return false, err
		}
		found = true
		break
	}
	if !found {
		return false, ErrSegmentNotFound.New("segment missing")
	}

	return altered, nil
}

// CheckSegmentPiecesAlteration checks if a segment exists and if its pieces match the provided alias pieces.
// It returns true if pieces don't match, otherwise false.
// The comparison is done at the database level for efficiency.
func (p *PostgresAdapter) CheckSegmentPiecesAlteration(ctx context.Context, streamID uuid.UUID, position SegmentPosition, aliasPieces AliasPieces) (altered bool, err error) {
	defer mon.Task()(&ctx)(&err)

	expectedBytes, err := aliasPieces.Bytes()
	if err != nil {
		return false, Error.New("unable to convert alias pieces to bytes: %w", err)
	}

	var isInline, piecesMatch bool
	err = p.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(LENGTH(remote_alias_pieces), 0) = 0 AS is_inline,
			COALESCE(remote_alias_pieces, ''::bytea) = $3
		FROM segments
		WHERE (stream_id, position) = ($1, $2)
	`, streamID, position.Encode(), expectedBytes).Scan(
		&isInline, &piecesMatch,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrSegmentNotFound.New("segment missing")
		}
		return false, Error.New("unable to query segment pieces: %w", err)
	}

	if isInline {
		return false, ErrInvalidRequest.New("segment (stream ID: %s, Position: %+v) is NOT remote", streamID, position)
	}

	return !piecesMatch, nil
}

// CheckSegmentPiecesAlteration checks if a segment exists and if its pieces match the provided alias pieces.
// It returns true if pieces don't match, otherwise false.
// The comparison is done at the database level for efficiency.
func (s *SpannerAdapter) CheckSegmentPiecesAlteration(ctx context.Context, streamID uuid.UUID, position SegmentPosition, aliasPieces AliasPieces) (altered bool, err error) {
	defer mon.Task()(&ctx)(&err)

	type result struct {
		isInline    bool
		piecesMatch bool
	}
	res, err := spannerutil.CollectRow(s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				COALESCE(LENGTH(remote_alias_pieces), 0) = 0 AS is_inline,
				COALESCE(remote_alias_pieces, CAST('' AS BYTES)) = @alias_pieces
			FROM segments
			WHERE stream_id = @stream_id AND position = @position
		`,
		Params: map[string]interface{}{
			"stream_id":    streamID,
			"position":     position,
			"alias_pieces": aliasPieces,
		},
	}, spanner.QueryOptions{RequestTag: "check-segment-pieces-alteration"}),
		func(row *spanner.Row, res *result) error {
			return row.Columns(&res.isInline, &res.piecesMatch)
		})

	if err != nil {
		if errors.Is(err, iterator.Done) {
			return false, ErrSegmentNotFound.New("segment missing")
		}
		return false, Error.New("unable to query segment pieces: %w", err)
	}

	if res.isInline {
		return false, ErrInvalidRequest.New("segment (stream ID: %s, Position: %+v) is NOT remote", streamID, position)
	}

	return !res.piecesMatch, nil
}
