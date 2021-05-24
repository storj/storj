// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"math"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// DeletePart contains arguments necessary for deleting single part.
type DeletePart struct {
	StreamID   uuid.UUID
	PartNumber uint32

	DeletePieces func(ctx context.Context, segment DeletedSegmentInfo) error
}

// DeletePart deletes all segments for given part.
func (db *DB) DeletePart(ctx context.Context, opts DeletePart) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}

	if opts.DeletePieces == nil {
		return ErrInvalidRequest.New("DeletePieces missing")
	}

	minPosition := SegmentPosition{
		Part: opts.PartNumber,
	}.Encode()
	maxPosition := SegmentPosition{
		Part:  opts.PartNumber,
		Index: math.MaxUint32,
	}.Encode()

	type Deleted struct {
		RootPieceID storj.PieceID
		AliasPieces AliasPieces
	}
	deleted := make([]Deleted, 0, 10)
	err = withRows(db.db.QueryContext(ctx, `
				DELETE FROM segments WHERE
					stream_id = $1 AND position BETWEEN $2 AND $3
				RETURNING
					root_piece_id, remote_alias_pieces
			`, opts.StreamID, minPosition, maxPosition))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var rootPieceID storj.PieceID
			var aliasPieces AliasPieces
			err := rows.Scan(&rootPieceID, &aliasPieces)
			if err != nil {
				return err
			}
			// this code assumes that at some point we will limit number of segments per part
			deleted = append(deleted, Deleted{
				RootPieceID: rootPieceID,
				AliasPieces: aliasPieces,
			})

		}
		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	for _, item := range deleted {
		deleteInfo := DeletedSegmentInfo{
			RootPieceID: item.RootPieceID,
		}
		deleteInfo.Pieces, err = db.aliasCache.ConvertAliasesToPieces(ctx, item.AliasPieces)
		if err != nil {
			return err
		}
		err = opts.DeletePieces(ctx, deleteInfo)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}
