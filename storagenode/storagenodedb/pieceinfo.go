// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
)

type pieceinfo struct{ *infodb }

// PieceInfo returns database for storing piece information
func (db *DB) PieceInfo() pieces.DB { return db.info.PieceInfo() }

// PieceInfo returns database for storing piece information
func (db *infodb) PieceInfo() pieces.DB { return &pieceinfo{db} }

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
func (db *pieceinfo) Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID2) (*pieces.Info, error) {
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

	info.Uplink, err = identity.PeerIdentityFromPEM(uplinkIdentity)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}

	return info, nil
}
