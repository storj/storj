// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ErrValueChanged is returned when the current value of the key does not match the oldValue in UpdateSegmentPieces.
var ErrValueChanged = errs.Class("value changed")

// UpdateSegmentPieces contains arguments necessary for updating segment pieces.
type UpdateSegmentPieces struct {
	StreamID uuid.UUID
	Position SegmentPosition

	OldPieces Pieces

	NewRedundancy storj.RedundancyScheme
	NewPieces     Pieces

	NewRepairedAt time.Time // sets new time of last segment repair (optional).
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

	if opts.NewRedundancy.IsZero() {
		return ErrInvalidRequest.New("NewRedundancy zero")
	}

	// its possible that in this method we will have less pieces
	// than optimal shares (e.g. after repair)
	if len(opts.NewPieces) < int(opts.NewRedundancy.RepairShares) {
		return ErrInvalidRequest.New("number of new pieces is less than new redundancy repair shares value")
	}

	if err := opts.NewPieces.Verify(); err != nil {
		if ErrInvalidRequest.Has(err) {
			return ErrInvalidRequest.New("NewPieces: %v", errs.Unwrap(err))
		}
		return err
	}

	updateRepairAt := !opts.NewRepairedAt.IsZero()

	oldPieces, err := db.aliasCache.EnsurePiecesToAliases(ctx, opts.OldPieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	newPieces, err := db.aliasCache.EnsurePiecesToAliases(ctx, opts.NewPieces)
	if err != nil {
		return Error.New("unable to convert pieces to aliases: %w", err)
	}

	var resultPieces AliasPieces
	err = db.db.QueryRowContext(ctx, `
		UPDATE segments SET
			remote_alias_pieces = CASE
				WHEN remote_alias_pieces = $3 THEN $4
				ELSE remote_alias_pieces
			END,
			redundancy = CASE
				WHEN remote_alias_pieces = $3 THEN $5
				ELSE redundancy
			END,
			repaired_at = CASE
				WHEN remote_alias_pieces = $3 AND $7 = true THEN $6
				ELSE repaired_at
			END
		WHERE
			stream_id     = $1 AND
			position      = $2
		RETURNING remote_alias_pieces
		`, opts.StreamID, opts.Position, oldPieces, newPieces, redundancyScheme{&opts.NewRedundancy}, opts.NewRepairedAt, updateRepairAt).
		Scan(&resultPieces)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSegmentNotFound.New("segment missing")
		}
		return Error.New("unable to update segment pieces: %w", err)
	}

	if !EqualAliasPieces(newPieces, resultPieces) {
		return ErrValueChanged.New("segment remote_alias_pieces field was changed")
	}

	mon.Meter("segment_update").Mark(1)

	return nil
}
