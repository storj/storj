// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// GetStreamPieceCountByNodeID contains arguments for GetStreamPieceCountByNodeID.
type GetStreamPieceCountByNodeID struct {
	StreamID uuid.UUID
}

// GetStreamPieceCountByNodeID returns piece count by node id.
func (db *DB) GetStreamPieceCountByNodeID(ctx context.Context, opts GetStreamPieceCountByNodeID) (result map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return nil, ErrInvalidRequest.New("StreamID missing")
	}

	countByAlias := map[NodeAlias]int64{}
	result = map[storj.NodeID]int64{}
	err = withRows(db.db.QueryContext(ctx, `
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

	for alias, count := range countByAlias {
		nodeID, err := db.aliasCache.Nodes(ctx, []NodeAlias{alias})
		if err != nil {
			return nil, Error.New("unable to convert aliases to pieces: %w", err)
		}
		result[nodeID[0]] = count
	}

	return result, nil
}
