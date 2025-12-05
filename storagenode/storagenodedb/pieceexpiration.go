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
func (db *pieceExpirationDB) GetExpired(ctx context.Context, now time.Time, opts pieces.ExpirationOptions) (info []*pieces.ExpiredInfoRecords, err error) {
	defer monGetExpired(&ctx)(&err)

	now = now.UTC()

	db.mu.Lock()
	count := 0
	infoRecordsBySatellite := make(map[storj.NodeID]*pieces.ExpiredInfoRecords)

	for ei, exp := range db.buf {
		if exp.Before(now) {
			satList, ok := infoRecordsBySatellite[ei.SatelliteID]
			if !ok {
				satList = pieces.NewExpiredInfoRecords(ei.SatelliteID, false, 1)
				infoRecordsBySatellite[ei.SatelliteID] = satList
				info = append(info, satList)
			}
			satList.Append(ei.PieceID, ei.PieceSize)
			count++
			if opts.Limits.BatchSize > 0 && count >= opts.Limits.BatchSize {
				break
			}
		}
	}
	db.mu.Unlock()

	// if we have enough pieces in the buffer, we don't need to query the database
	if opts.Limits.BatchSize > 0 && count >= opts.Limits.BatchSize {
		return info, nil
	}

	opts.Limits.BatchSize -= count
	expiredFromDB, err := db.getExpiredPaginated(ctx, now, opts.Limits.BatchSize, opts.ReverseOrder)
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
func (db *pieceExpirationDB) getExpiredPaginated(ctx context.Context, now time.Time, limit int, reverse bool) (info []*pieces.ExpiredInfoRecords, err error) {
	defer monGetExpiredPaginated(&ctx)(&err)

	order := "ASC"
	if reverse {
		order = "DESC"
	}

	query := `
		SELECT satellite_id, piece_id
		FROM piece_expirations
		WHERE piece_expiration < ?
		ORDER BY piece_expiration ` + order

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

	infoRecordsBySatellite := make(map[storj.NodeID]*pieces.ExpiredInfoRecords)
	for rows.Next() {
		var ei pieces.ExpiredInfo
		err = rows.Scan(&ei.SatelliteID, &ei.PieceID)
		if err != nil {
			return nil, ErrPieceExpiration.Wrap(err)
		}
		satList, ok := infoRecordsBySatellite[ei.SatelliteID]
		if !ok {
			satList = pieces.NewExpiredInfoRecords(ei.SatelliteID, false, 1)
			info = append(info, satList)
			infoRecordsBySatellite[ei.SatelliteID] = satList
		}
		satList.Append(ei.PieceID, ei.PieceSize)
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
func (db *pieceExpirationDB) DeleteExpirationsBatch(ctx context.Context, now time.Time, opts pieces.ExpirationOptions) (err error) {
	defer monDeleteExpirationsBatch(&ctx)(&err)

	if opts.Limits.BatchSize <= 0 {
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

	order := "ASC"
	if opts.ReverseOrder {
		order = "DESC"
	}

	_, err = db.ExecContext(ctx, `
		DELETE FROM piece_expirations
			WHERE (satellite_id, piece_id) IN (
				SELECT satellite_id, piece_id
				FROM piece_expirations
				WHERE piece_expiration < ?
				ORDER BY piece_expiration `+order+`
				LIMIT ?
			)
	`, now, opts.Limits.BatchSize)

	return ErrPieceExpiration.Wrap(err)
}
