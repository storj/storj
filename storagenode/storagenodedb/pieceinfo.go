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

type pieceinfo struct{ *InfoDB }

// PieceInfo returns database for storing piece information
func (db *DB) PieceInfo() pieces.DB { return db.info.PieceInfo() }

// PieceInfo returns database for storing piece information
func (db *InfoDB) PieceInfo() pieces.DB { return &pieceinfo{db} }

// Add inserts piece information into the database.
func (db *pieceinfo) Add(ctx context.Context, info *pieces.Info) (err error) {
	defer mon.Task()(&ctx)(&err)
	certdb := db.CertDB()
	certid, err := certdb.Include(ctx, info.Uplink)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	uplinkPieceHash, err := proto.Marshal(info.UplinkPieceHash)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	_, err = db.db.ExecContext(ctx, db.Rebind(`
		INSERT INTO
			pieceinfo(satellite_id, piece_id, piece_size, piece_expiration, uplink_piece_hash, uplink_cert_id)
		VALUES (?,?,?,?,?,?)
	`), info.SatelliteID, info.PieceID, info.PieceSize, info.PieceExpiration, uplinkPieceHash, certid)

	return ErrInfo.Wrap(err)
}

// Get gets piece information by satellite id and piece id.
func (db *pieceinfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (_ *pieces.Info, err error) {
	defer mon.Task()(&ctx)(&err)
	info := &pieces.Info{}
	info.SatelliteID = satelliteID
	info.PieceID = pieceID

	var uplinkPieceHash []byte
	var uplinkIdentity []byte

	err = db.db.QueryRowContext(ctx, db.Rebind(`
		SELECT piece_size, piece_expiration, uplink_piece_hash, certificate.peer_identity
		FROM pieceinfo
		INNER JOIN certificate ON pieceinfo.uplink_cert_id = certificate.cert_id
		WHERE satellite_id = ? AND piece_id = ?
	`), satelliteID, pieceID).Scan(&info.PieceSize, &info.PieceExpiration, &uplinkPieceHash, &uplinkIdentity)

	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	info.UplinkPieceHash = &pb.PieceHash{}
	err = proto.Unmarshal(uplinkPieceHash, info.UplinkPieceHash)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	info.Uplink, err = decodePeerIdentity(ctx, uplinkIdentity)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return info, nil
}

// Delete deletes piece information.
func (db *pieceinfo) Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.ExecContext(ctx, db.Rebind(`
		DELETE FROM pieceinfo
		WHERE satellite_id = ?
		  AND piece_id = ?
	`), satelliteID, pieceID)

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
		WHERE piece_expiration < ? AND ((deletion_failed_at IS NULL) OR deletion_failed_at <> ?)
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

// SpaceUsed calculates disk space used by all pieces
func (db *pieceinfo) SpaceUsed(ctx context.Context) (_ int64, err error) {
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
