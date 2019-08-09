// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

func (certs *certDB) Set(ctx context.Context, nodeID storj.NodeID, pi *identity.PeerIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := certs.db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	if pi == nil {
		return Error.New("Peer Identity cannot be nil")
	}
	chain := identity.EncodePeerIdentity(pi)

	var id []byte
	query := `SELECT node_id FROM peerIdentities WHERE serial_number = ?;`
	err = tx.QueryRow(certs.db.Rebind(query), pi.Leaf.SerialNumber.Bytes()).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			// create a new entry
			_, err = tx.Exec(certs.db.Rebind(`INSERT INTO peerIdentities ( serial_number, peer_identity, node_id, update_at ) VALUES ( ?, ?, ?, ? );`), pi.Leaf.SerialNumber.Bytes(), chain, nodeID.Bytes(), time.Now())
			if err != nil {
				return Error.Wrap(err)
			}
			return nil
		}
		return Error.Wrap(err)
	}

	// already public key exists, just return
	return nil
}

// Get gets the public key based on the certificate's serial number
func (certs *certDB) Get(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := certs.db.Get_PeerIdentity_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if dbxInfo == nil {
		return nil, Error.New("unknown nodeID :%+v: %+v", nodeID.Bytes(), err)
	}

	peer, err := identity.DecodePeerIdentity(ctx, dbxInfo.PeerIdentity)
	return peer, Error.Wrap(err)
}

// BatchGet gets the public key based on the certificate's serial number
func (certs *certDB) BatchGet(ctx context.Context, nodeIDs []storj.NodeID) (peers []*identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	tx, err := certs.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	for _, nodeID := range nodeIDs {
		dbxInfo, err := tx.Get_PeerIdentity_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
		if err != nil {
			return nil, errs.Combine(err, tx.Rollback())
		}

		if dbxInfo == nil {
			return nil, errs.Combine(Error.New("unknown nodeID :%+v: %+v", nodeID.Bytes(), err), tx.Rollback())
		}

		peer, err := identity.DecodePeerIdentity(ctx, dbxInfo.PeerIdentity)
		peers = append(peers, peer)
	}
	return peers, Error.Wrap(tx.Commit())
}
