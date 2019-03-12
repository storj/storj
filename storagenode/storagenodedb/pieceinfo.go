// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

type pieceinfo struct {
	*infodb
}

// PieceInfo returns piece info database
func (db *infodb) PieceInfo() pieceinfo { return pieceinfo{db} }

// Add inserts piece information into the database.
func (db *pieceinfo) Add(ctx context.Context, info pieces.Info) error {
	certdb := db.CertDB()
	certid, err := certdb.Include(ctx, info.UplinkID, info.UplinkCert)
	if err != nil {
		return ErrInfo.Wrap(err)
	}

	defer db.locked()()

	_, err = db.db.Exec(`
		INSERT INTO
			pieceinfo(satellite_id, piece_id, piece_size, piece_expiration, uplink_piece_hash, uplink_cert_id)
		VALUES (?,?,?,?,?,?)
	`, info.SatelliteID.Bytes(), info.PieceID.Bytes(), info.PieceSize, info.UplinkPieceHash, certid)

	return ErrInfo.Wrap(err)
}

// Get gets piece information by satellite id and piece id.
func (db *pieceinfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID2) (pieces.Info, error) {
	defer db.locked()()

	var info pieces.Info
	info.SatelliteID = satelliteID
	info.PieceID = pieceID

	err := db.db.QueryRow(`
		SELECT piece_size, piece_expiration, uplink_piece_hash, certificate.node_id, certificate.pkix
		FROM pieceinfo
		INNER JOIN certificate ON pieceinfo.uplink_cert_id = certificate.cert_id
	`).Scan(&info.PieceSize, &info.PieceExpiration, &info.UplinkPieceHash, &info.UplinkID, &info.UplinkCert)

	return info, ErrInfo.Wrap(err)
}
