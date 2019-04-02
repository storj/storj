// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"github.com/gogo/protobuf/proto"

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
func (db *pieceinfo) Add(ctx context.Context, info *pieces.Info) error {
	certdb := db.CertDB()
	certid, err := certdb.Include(ctx, info.Uplink)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	uplinkPieceHash, err := proto.Marshal(info.UplinkPieceHash)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	defer db.locked()()

	_, err = db.db.Exec(`
		INSERT INTO
			pieceinfo(satellite_id, piece_id, piece_size, piece_expiration, uplink_piece_hash, uplink_cert_id)
		VALUES (?,?,?,?,?,?)
	`, info.SatelliteID, info.PieceID, info.PieceSize, info.PieceExpiration, uplinkPieceHash, certid)

	return ErrInfo.Wrap(err)
}

// Get gets piece information by satellite id and piece id.
func (db *pieceinfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*pieces.Info, error) {
	info := &pieces.Info{}
	info.SatelliteID = satelliteID
	info.PieceID = pieceID

	var uplinkPieceHash []byte
	var uplinkIdentity []byte

	db.mu.Lock()
	err := db.db.QueryRow(`
		SELECT piece_size, piece_expiration, uplink_piece_hash, certificate.peer_identity
		FROM pieceinfo
		INNER JOIN certificate ON pieceinfo.uplink_cert_id = certificate.cert_id
		WHERE satellite_id = ? AND piece_id = ?
	`, satelliteID, pieceID).Scan(&info.PieceSize, &info.PieceExpiration, &uplinkPieceHash, &uplinkIdentity)
	db.mu.Unlock()

	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	info.UplinkPieceHash = &pb.PieceHash{}
	err = proto.Unmarshal(uplinkPieceHash, info.UplinkPieceHash)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	info.Uplink, err = decodePeerIdentity(uplinkIdentity)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return info, nil
}

// Delete deletes piece information.
func (db *pieceinfo) Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error {
	defer db.locked()()

	_, err := db.db.Exec(`DELETE FROM pieceinfo WHERE satellite_id = ? AND piece_id = ?`, satelliteID, pieceID)

	return ErrInfo.Wrap(err)
}

// SpaceUsed calculates disk space used by all pieces
func (db *pieceinfo) SpaceUsed(ctx context.Context) (int64, error) {
	defer db.locked()()

	var sum *int64
	err := db.db.QueryRow(`SELECT SUM(piece_size) FROM pieceinfo;`).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// GetExpired gets orders that are expired and were created before some time
func (db *pieceinfo) GetExpired(ctx context.Context, expiredAt time.Time) (info []pieces.Info, err error) {
	var getExpiredSQL = `SELECT satellite_id, piece_id, piece_size, piece_expiration, uplink_piece_hash, uplink_cert_id 
		FROM pieceinfo WHERE piece_expiration < ?`
	rows, err := db.db.QueryContext(ctx, db.Rebind(getExpiredSQL), expiredAt)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for i := 0; rows.Next(); i++ {
		pi := pieces.Info{}
		err = rows.Scan(&pi.SatelliteID, &pi.PieceID, &pi.PieceSize, &pi.PieceExpiration, &pi.UplinkPieceHash, &pi.Uplink)
		if err != nil {
			break
		}
		info = append(info, pi)
	}
	return info, err
}

// DeleteExpired deletes expired piece information.
func (db *pieceinfo) DeleteExpired(ctx context.Context, expiredAt time.Time) error {
	defer db.locked()()

	_, err := db.db.Exec(`DELETE FROM pieceinfo WHERE piece_expiration < ?`, expiredAt)

	return ErrInfo.Wrap(err)
}
