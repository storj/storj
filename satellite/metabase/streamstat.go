// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"cloud.google.com/go/spanner"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
)

// GetStreamPieceCountByNodeID contains arguments for GetStreamPieceCountByNodeID.
type GetStreamPieceCountByNodeID struct {
	ProjectID uuid.UUID
	StreamID  uuid.UUID
}

// GetStreamPieceCountByNodeID returns piece count by node id.
func (db *DB) GetStreamPieceCountByNodeID(ctx context.Context, opts GetStreamPieceCountByNodeID) (result map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return nil, ErrInvalidRequest.New("StreamID missing")
	}

	result = map[storj.NodeID]int64{}
	countByAlias, err := db.ChooseAdapter(opts.ProjectID).GetStreamPieceCountByAlias(ctx, opts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return result, Error.New("unable to fetch object segments: %w", err)
	}

	for alias, count := range countByAlias {
		nodeID, err := db.aliasCache.Nodes(ctx, []NodeAlias{alias})
		if err != nil {
			return nil, Error.New("unable to convert aliases to pieces: %w", err)
		}
		result[nodeID[0]] = count
	}

	return result, nil
}

// GetStreamPieceCountByAlias returns piece count by node alias.
func (p *PostgresAdapter) GetStreamPieceCountByAlias(ctx context.Context, opts GetStreamPieceCountByNodeID) (result map[NodeAlias]int64, err error) {
	countByAlias := map[NodeAlias]int64{}
	err = withRows(p.db.QueryContext(ctx, `
		SELECT remote_alias_pieces
		FROM   segments
		WHERE  stream_id = $1 AND remote_alias_pieces IS NOT null
	`, opts.StreamID))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var aliasPieces AliasPieces
			err = rows.Scan(&aliasPieces)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			for i := range aliasPieces {
				countByAlias[aliasPieces[i].Alias]++
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return result, Error.New("unable to fetch object segments: %w", err)
	}

	return countByAlias, nil
}

// GetStreamPieceCountByAlias returns piece count by node alias.
func (s *SpannerAdapter) GetStreamPieceCountByAlias(ctx context.Context, opts GetStreamPieceCountByNodeID) (result map[NodeAlias]int64, err error) {
	countByAlias := map[NodeAlias]int64{}
	err = s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT remote_alias_pieces
			FROM   segments
			WHERE  stream_id = @stream_id AND remote_alias_pieces IS NOT null
		`,
		Params: map[string]interface{}{
			"stream_id": opts.StreamID,
		},
	}, spanner.QueryOptions{RequestTag: "get-stream-piece-count-by-alias"}).Do(
		func(row *spanner.Row) error {
			var aliasPieces AliasPieces
			err = row.Columns(&aliasPieces)
			if err != nil {
				return Error.New("failed to scan segments: %w", err)
			}

			for i := range aliasPieces {
				countByAlias[aliasPieces[i].Alias]++
			}
			return nil
		})
	if err != nil {
		return result, Error.New("unable to fetch object segments: %w", err)
	}

	return countByAlias, nil
}
