// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

type v0PieceInfo struct {
	*InfoDB
}

// V0PieceInfo returns database for storing piece information
func (db *DB) V0PieceInfo() pieces.V0PieceInfoDB { return db.info.V0PieceInfo() }

// V0PieceInfo returns database for storing piece information
func (db *InfoDB) V0PieceInfo() pieces.V0PieceInfoDB { return &db.v0PieceInfo }

// Add inserts piece information into the database.
func (db *v0PieceInfo) Add(ctx context.Context, info *pieces.Info) (err error) {
	defer mon.Task()(&ctx)(&err)

	orderLimit, err := proto.Marshal(info.OrderLimit)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	uplinkPieceHash, err := proto.Marshal(info.UplinkPieceHash)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	var pieceExpiration *time.Time
	if !info.PieceExpiration.IsZero() {
		utcExpiration := info.PieceExpiration.UTC()
		pieceExpiration = &utcExpiration
	}

	// TODO remove `uplink_cert_id` from DB
	_, err = db.db.ExecContext(ctx, db.Rebind(`
		INSERT INTO
			pieceinfo_(satellite_id, piece_id, piece_size, piece_creation, piece_expiration, order_limit, uplink_piece_hash, uplink_cert_id)
		VALUES (?,?,?,?,?,?,?,?)
	`), info.SatelliteID, info.PieceID, info.PieceSize, info.PieceCreation.UTC(), pieceExpiration, orderLimit, uplinkPieceHash, 0)

	return ErrInfo.Wrap(err)
}

// ForAllV0PieceIDsOwnedBySatellite executes doForEach for each locally stored piece, stored with
// storage format V0 in the namespace of the given satellite, if that piece was created before
// the specified time. If doForEach returns a non-nil error, ForAllV0PieceIDsOwnedBySatellite will
// stop iterating and return the error immediately.
func (db *v0PieceInfo) ForAllV0PieceIDsOwnedBySatellite(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, doForEach func(storj.PieceID) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT piece_id, piece_size, piece_creation, piece_expiration, order_limit, uplink_piece_hash
		FROM pieceinfo_
		WHERE satellite_id = ? AND datetime(piece_creation) < datetime(?)
		ORDER BY piece_id
	`), satelliteID, createdBefore.UTC())
	if err != nil {
		return ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var pieceID storj.PieceID
		var expirationTime *time.Time
		err = rows.Scan(&pieceID, &expirationTime)
		if err != nil {
			return ErrInfo.Wrap(err)
		}
		if err := doForEach(pieceID); err != nil {
			return err
		}
	}
	return nil
}

// Get gets piece information by satellite id and piece id.
func (db *v0PieceInfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (_ *pieces.Info, err error) {
	defer mon.Task()(&ctx)(&err)
	info := &pieces.Info{}
	info.SatelliteID = satelliteID
	info.PieceID = pieceID

	var orderLimit []byte
	var uplinkPieceHash []byte
	var pieceExpiration *time.Time

	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT piece_size, piece_creation, piece_expiration, order_limit, uplink_piece_hash
		FROM pieceinfo_
		WHERE satellite_id = ? AND piece_id = ?
	`), satelliteID, pieceID).Scan(&info.PieceSize, &info.PieceCreation, &pieceExpiration, &orderLimit, &uplinkPieceHash)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	if pieceExpiration != nil {
		info.PieceExpiration = *pieceExpiration
	}

	info.OrderLimit = &pb.OrderLimit{}
	err = proto.Unmarshal(orderLimit, info.OrderLimit)
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
func (db *v0PieceInfo) Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var pieceSize int64
	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT piece_size
		FROM pieceinfo_
		WHERE satellite_id = ? AND piece_id = ?
	`), satelliteID, pieceID).Scan(&pieceSize)
	// Ignore no rows found errors
	if err != nil && err != sql.ErrNoRows {
		return ErrInfo.Wrap(err)
	}
	_, err = db.db.ExecContext(ctx, db.Rebind(`
		DELETE FROM pieceinfo_
		WHERE satellite_id = ?
		  AND piece_id = ?
	`), satelliteID, pieceID)

	return ErrInfo.Wrap(err)
}

// DeleteFailed marks piece as a failed deletion.
func (db *v0PieceInfo) DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.ExecContext(ctx, db.Rebind(`
		UPDATE pieceinfo_
		SET deletion_failed_at = ?
		WHERE satellite_id = ?
		  AND piece_id = ?
	`), now.UTC(), satelliteID, pieceID)

	return ErrInfo.Wrap(err)
}

// GetExpired gets ExpiredInfo records for pieces that are expired.
func (db *v0PieceInfo) GetExpired(ctx context.Context, expiredAt time.Time, limit int64) (infos []pieces.ExpiredInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT satellite_id, piece_id, piece_size
		FROM pieceinfo_
		WHERE piece_expiration IS NOT NULL
		AND piece_expiration < ?
		AND ((deletion_failed_at IS NULL) OR deletion_failed_at <> ?)
		ORDER BY satellite_id
		LIMIT ?
	`), expiredAt.UTC(), expiredAt.UTC(), limit)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		info := pieces.ExpiredInfo{InPieceInfo: true}
		err = rows.Scan(&info.SatelliteID, &info.PieceID, &info.PieceSize)
		if err != nil {
			return infos, ErrInfo.Wrap(err)
		}
		infos = append(infos, info)
	}
	return infos, nil
}
