// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

var (
	// ErrPieceExpiration represents errors from the piece expiration database.
	ErrPieceExpiration = errs.Class("pieceexpirationdb")

	// MaxPieceExpirationBufferSize is the maximum number of pieces that can be stored in the buffer before
	// they are written to the database.
	MaxPieceExpirationBufferSize = 1000 // TODO: make this configurable and set it at the top level.
)

// PieceExpirationDBName represents the database filename.
const PieceExpirationDBName = "piece_expiration"

type pieceExpirationDB struct {
	dbContainerImpl

	mu  sync.Mutex
	buf map[pieces.ExpiredInfo]time.Time
}

var monGetExpired = mon.Task()

// GetExpired gets piece IDs that expire or have expired before the given time.
// If batchSize is less than or equal to 0, it will return all expired pieces in one batch.
func (db *pieceExpirationDB) GetExpired(ctx context.Context, now time.Time, batchSize int) (info []pieces.ExpiredInfo, err error) {
	defer monGetExpired(&ctx)(&err)

	now = now.UTC()

	db.mu.Lock()
	count := 0
	for ei, exp := range db.buf {
		if exp.Before(now) {
			info = append(info, ei)
			count++
			if batchSize > 0 && count >= batchSize {
				break
			}
		}
	}
	db.mu.Unlock()

	// if we have enough pieces in the buffer, we don't need to query the database
	if batchSize > 0 && count >= batchSize {
		return info, nil
	}

	batchSize -= count
	expiredFromDB, err := db.getExpiredPaginated(ctx, now, batchSize)
	if err != nil {
		return nil, err
	}

	if len(expiredFromDB) == 0 {
		return info, nil
	}

	return append(info, expiredFromDB...), nil
}

var monGetExpiredPaginated = mon.Task()

// getExpiredPaginated returns a paginated list of expired pieces.
// If limit is less than or equal to 0, it will return all expired pieces.
func (db *pieceExpirationDB) getExpiredPaginated(ctx context.Context, now time.Time, limit int) (info []pieces.ExpiredInfo, err error) {
	defer monGetExpiredPaginated(&ctx)(&err)

	query := `
		SELECT satellite_id, piece_id
		FROM piece_expirations
		WHERE piece_expiration < ?
		ORDER BY piece_expiration
	`

	var args = []interface{}{now.UTC()}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return info, nil
		}
		return nil, ErrPieceExpiration.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Err(), rows.Close())
	}()

	for rows.Next() {
		var ei pieces.ExpiredInfo
		err = rows.Scan(&ei.SatelliteID, &ei.PieceID)
		if err != nil {
			return nil, ErrPieceExpiration.Wrap(err)
		}
		info = append(info, ei)
	}

	return info, nil
}

var monSetExpiration = mon.Task()

// SetExpiration sets an expiration time for the given piece ID on the given satellite.
func (db *pieceExpirationDB) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time, pieceSize int64) (err error) {
	defer monSetExpiration(&ctx)(&err)

	ei := pieces.ExpiredInfo{
		SatelliteID: satellite,
		PieceID:     pieceID,
	}

	db.mu.Lock()
	if db.buf == nil {
		db.buf = make(map[pieces.ExpiredInfo]time.Time, 1000)
	}
	db.buf[ei] = expiresAt.UTC()
	if len(db.buf) < MaxPieceExpirationBufferSize {
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

// DeleteExpirations removes expiration records for pieces that have expired before the given time.
func (db *pieceExpirationDB) DeleteExpirations(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now = now.UTC()

	db.mu.Lock()
	for ei, exp := range db.buf {
		if exp.Before(now) {
			delete(db.buf, ei)
		}
	}
	db.mu.Unlock()

	_, err = db.ExecContext(ctx, `
		DELETE FROM piece_expirations
			WHERE piece_expiration < ?
	`, now)
	return ErrPieceExpiration.Wrap(err)
}

var monDeleteExpirationsBatch = mon.Task()

// DeleteExpirationsBatch removes expiration records for pieces that have expired before the given time
// and falls within the limit.
// If limit is less than or equal to 0, it will delete all expired pieces.
func (db *pieceExpirationDB) DeleteExpirationsBatch(ctx context.Context, now time.Time, limit int) (err error) {
	defer monDeleteExpirationsBatch(&ctx)(&err)

	if limit <= 0 {
		return db.DeleteExpirations(ctx, now)
	}

	now = now.UTC()

	db.mu.Lock()
	for ei, exp := range db.buf {
		if exp.Before(now) {
			delete(db.buf, ei)
		}
	}
	db.mu.Unlock()

	_, err = db.ExecContext(ctx, `
		DELETE FROM piece_expirations
			WHERE (satellite_id, piece_id) IN (
				SELECT satellite_id, piece_id
				FROM piece_expirations
				WHERE piece_expiration < ?
				ORDER BY piece_expiration
				LIMIT ?
			)
	`, now, limit)

	return ErrPieceExpiration.Wrap(err)
}
