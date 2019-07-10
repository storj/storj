// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

type pieceinfo struct {
	*InfoDB
	space spaceUsed
}

type spaceUsed struct {
	// Moved to top of struct to resolve alignment issue with atomic operations on ARM
	used int64
	once sync.Once
}

// PieceInfo returns database for storing piece information
func (db *DB) PieceInfo() pieces.DB { return db.info.PieceInfo() }

// PieceInfo returns database for storing piece information
func (db *InfoDB) PieceInfo() pieces.DB { return &db.pieceinfo }

// Add inserts piece information into the database.
func (db *pieceinfo) Add(ctx context.Context, info *pieces.Info) (err error) {
	defer mon.Task()(&ctx)(&err)

	uplinkPieceHash, err := proto.Marshal(info.UplinkPieceHash)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	// TODO remove `uplink_cert_id` from DB
	_, err = db.db.ExecContext(ctx, db.Rebind(`
		INSERT INTO
			pieceinfo(satellite_id, piece_id, piece_size, piece_creation, piece_expiration, uplink_piece_hash, uplink_cert_id)
		VALUES (?,?,?,?,?,?,?)
	`), info.SatelliteID, info.PieceID, info.PieceSize, info.PieceCreation, info.PieceExpiration, uplinkPieceHash, 0)

	if err == nil {
		db.loadSpaceUsed(ctx)
		atomic.AddInt64(&db.space.used, info.PieceSize)
	}
	return ErrInfo.Wrap(err)
}

// GetPieceIDs gets pieceIDs using the satelliteID
func (db *pieceinfo) GetPieceIDs(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, limit, offset int) (pieceIDs []storj.PieceID, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT piece_id
		FROM pieceinfo
		WHERE satellite_id = ? AND piece_creation < ?
		ORDER BY piece_id
		LIMIT ? OFFSET ?
	`), satelliteID, createdBefore, limit, offset)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var pieceID storj.PieceID
		err = rows.Scan(&pieceID)
		if err != nil {
			return pieceIDs, ErrInfo.Wrap(err)
		}
		pieceIDs = append(pieceIDs, pieceID)
	}
	return pieceIDs, nil
}

// Get gets piece information by satellite id and piece id.
func (db *pieceinfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (_ *pieces.Info, err error) {
	defer mon.Task()(&ctx)(&err)
	info := &pieces.Info{}
	info.SatelliteID = satelliteID
	info.PieceID = pieceID

	var uplinkPieceHash []byte

	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT piece_size, piece_creation, piece_expiration, uplink_piece_hash
		FROM pieceinfo
		WHERE satellite_id = ? AND piece_id = ?
	`), satelliteID, pieceID).Scan(&info.PieceSize, &info.PieceCreation, &info.PieceExpiration, &uplinkPieceHash)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	info.UplinkPieceHash = &pb.PieceHash{}
	err = proto.Unmarshal(uplinkPieceHash, info.UplinkPieceHash)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return info, nil
}

// Delete deletes piece information.
func (db *pieceinfo) Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var pieceSize int64
	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT piece_size
		FROM pieceinfo
		WHERE satellite_id = ? AND piece_id = ?
	`), satelliteID, pieceID).Scan(&pieceSize)
	// Ignore no rows found errors
	if err != nil && err != sql.ErrNoRows {
		return ErrInfo.Wrap(err)
	}
	_, err = db.db.ExecContext(ctx, db.Rebind(`
		DELETE FROM pieceinfo
		WHERE satellite_id = ?
		  AND piece_id = ?
	`), satelliteID, pieceID)

	if pieceSize != 0 && err == nil {
		db.loadSpaceUsed(ctx)

		atomic.AddInt64(&db.space.used, -pieceSize)
	}

	return ErrInfo.Wrap(err)
}

// DeleteFailed marks piece as a failed deletion.
func (db *pieceinfo) DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.ExecContext(ctx, db.Rebind(`
		UPDATE pieceinfo
		SET deletion_failed_at = ?
		WHERE satellite_id = ?
		  AND piece_id = ?
	`), now, satelliteID, pieceID)

	return ErrInfo.Wrap(err)
}

// GetExpired gets pieceinformation identites that are expired.
func (db *pieceinfo) GetExpired(ctx context.Context, expiredAt time.Time, limit int64) (infos []pieces.ExpiredInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT satellite_id, piece_id, piece_size
		FROM pieceinfo
		WHERE datetime(piece_expiration) < datetime(?) AND ((deletion_failed_at IS NULL) OR datetime(deletion_failed_at) <> datetime(?))
		ORDER BY satellite_id
		LIMIT ?
	`), expiredAt, expiredAt, limit)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		info := pieces.ExpiredInfo{}
		err = rows.Scan(&info.SatelliteID, &info.PieceID, &info.PieceSize)
		if err != nil {
			return infos, ErrInfo.Wrap(err)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// SpaceUsed returns disk space used by all pieces from cache
func (db *pieceinfo) SpaceUsed(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	db.loadSpaceUsed(ctx)

	return atomic.LoadInt64(&db.space.used), nil
}

func (db *pieceinfo) loadSpaceUsed(ctx context.Context) {
	defer mon.Task()(&ctx)
	db.space.once.Do(func() {
		usedSpace, _ := db.CalculatedSpaceUsed(ctx)
		atomic.AddInt64(&db.space.used, usedSpace)
	})
}

// CalculatedSpaceUsed calculates disk space used by all pieces
func (db *pieceinfo) CalculatedSpaceUsed(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	var sum sql.NullInt64
	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT SUM(piece_size)
		FROM pieceinfo
	`)).Scan(&sum)

	if err == sql.ErrNoRows || !sum.Valid {
		return 0, nil
	}
	return sum.Int64, err
}

// SpaceUsed calculates disk space used by all pieces
func (db *pieceinfo) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var sum sql.NullInt64
	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT SUM(piece_size)
		FROM pieceinfo
		WHERE satellite_id = ?
	`), satelliteID).Scan(&sum)

	if err == sql.ErrNoRows || !sum.Valid {
		return 0, nil
	}
	return sum.Int64, err
}
