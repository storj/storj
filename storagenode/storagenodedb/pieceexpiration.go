// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

// ErrPieceExpiration represents errors from the piece expiration database.
var ErrPieceExpiration = errs.Class("pieceexpirationdb")

// PieceExpirationDBName represents the database filename.
const PieceExpirationDBName = "piece_expiration"

type pieceExpirationDB struct {
	dbContainerImpl

	mu  sync.Mutex
	buf map[pieces.ExpiredInfo]time.Time
}

// GetExpired gets piece IDs that expire or have expired before the given time.
func (db *pieceExpirationDB) GetExpired(ctx context.Context, now time.Time, cb func(context.Context, pieces.ExpiredInfo) bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	db.mu.Lock()
	buf := map[pieces.ExpiredInfo]struct{}{}
	for ei, exp := range db.buf {
		if exp.Before(now) {
			buf[ei] = struct{}{}
		}
	}
	rows, err := db.QueryContext(ctx, `
		SELECT satellite_id, piece_id
			FROM piece_expirations
			WHERE piece_expiration < ?
	`, now.UTC())
	db.mu.Unlock()

	if err != nil {
		return ErrPieceExpiration.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

	for ei := range buf {
		if !cb(ctx, ei) {
			return nil
		}
	}

	for rows.Next() {
		var ei pieces.ExpiredInfo
		err = rows.Scan(&ei.SatelliteID, &ei.PieceID)
		if err != nil {
			return ErrPieceExpiration.Wrap(err)
		}
		if _, ok := buf[ei]; ok {
			continue
		}
		if !cb(ctx, ei) {
			return nil
		}
	}
	return nil
}

// SetExpiration sets an expiration time for the given piece ID on the given satellite.
func (db *pieceExpirationDB) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	ei := pieces.ExpiredInfo{
		SatelliteID: satellite,
		PieceID:     pieceID,
	}

	db.mu.Lock()
	if db.buf == nil {
		db.buf = make(map[pieces.ExpiredInfo]time.Time, 1000)
	}
	db.buf[ei] = expiresAt.UTC()
	if len(db.buf) < 1000 {
		db.mu.Unlock()
		return nil
	}

	var args []any
	for ei, expiresAt := range db.buf {
		args = append(args, ei.SatelliteID, ei.PieceID, expiresAt)
	}

	// done in separate loop so runtime optimizes to map clear call
	for ei := range db.buf {
		delete(db.buf, ei)
	}
	db.mu.Unlock()

	values := strings.TrimRight(strings.Repeat("(?,?,?),", len(args)/3), ",")
	_, err = db.ExecContext(ctx, `
		INSERT INTO piece_expirations (satellite_id, piece_id, piece_expiration) VALUES `+values,
		args...,
	)
	return ErrPieceExpiration.Wrap(err)
}

// DeleteExpiration removes an expiration record for the given piece ID on the given satellite.
func (db *pieceExpirationDB) DeleteExpirations(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	db.mu.Lock()
	defer db.mu.Unlock()

	for ei, exp := range db.buf {
		if exp.Before(now) {
			delete(db.buf, ei)
		}
	}

	_, err = db.ExecContext(ctx, `
		DELETE FROM piece_expirations
			WHERE piece_expiration < ?
	`, now.UTC())
	return ErrPieceExpiration.Wrap(err)
}
