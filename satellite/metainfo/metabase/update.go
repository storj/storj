// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/storage"
)

// UpdateSegmentPieces contains arguments necessary for updating segment pieces.
type UpdateSegmentPieces struct {
	StreamID uuid.UUID
	Position SegmentPosition

	OldPieces Pieces
	NewPieces Pieces
}

// UpdateSegmentPieces updates pieces for specified segment. If provided old pieces
// won't match current database state update will fail.
func (db *DB) UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.StreamID.IsZero() {
		return ErrInvalidRequest.New("StreamID missing")
	}

	if err := opts.OldPieces.Verify(); err != nil {
		if ErrInvalidRequest.Has(err) {
			return ErrInvalidRequest.New("OldPieces: %v", errs.Unwrap(err))
		}
		return err
	}

	if err := opts.NewPieces.Verify(); err != nil {
		if ErrInvalidRequest.Has(err) {
			return ErrInvalidRequest.New("NewPieces: %v", errs.Unwrap(err))
		}
		return err
	}

	oldPieces, err := db.aliasCache.ConvertPiecesToAliases(ctx, opts.OldPieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	newPieces, err := db.aliasCache.ConvertPiecesToAliases(ctx, opts.NewPieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	var resultPieces AliasPieces
	err = db.db.QueryRow(ctx, `
		UPDATE segments SET
			remote_alias_pieces = CASE
				WHEN remote_alias_pieces = $3 THEN $4
				ELSE remote_alias_pieces
			END
		WHERE
			stream_id     = $1 AND
			position      = $2
		RETURNING remote_alias_pieces
		`, opts.StreamID, opts.Position, oldPieces, newPieces).Scan(&resultPieces)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSegmentNotFound.New("segment missing")
		}
		return Error.New("unable to update segment pieces: %w", err)
	}

	if !EqualAliasPieces(newPieces, resultPieces) {
		return storage.ErrValueChanged.New("segment remote_alias_pieces field was changed")
	}

	return nil
}
