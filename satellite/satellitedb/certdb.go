// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

// Set adds a peer identity entry
func (certs *certDB) Set(ctx context.Context, nodeID storj.NodeID, peerIdent *identity.PeerIdentity) (err error) {
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

	if peerIdent == nil {
		return Error.New("Peer Identity cannot be nil")
	}
	chain := identity.EncodePeerIdentity(peerIdent)

	var id []byte
	query := `SELECT node_id FROM peer_identities WHERE serial_number = ?;`
	err = tx.QueryRow(certs.db.Rebind(query), peerIdent.Leaf.SerialNumber.Bytes()).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			// when storagenode's leaf certificate's serial number changes from perious,
			// a new entry will be made for the same storagenode ID
			_, err = tx.Exec(certs.db.Rebind(`INSERT INTO peer_identities ( serial_number, peer_identity, node_id, update_at ) VALUES ( ?, ?, ?, ? );`), peerIdent.Leaf.SerialNumber.Bytes(), chain, nodeID.Bytes(), time.Now())
			if err != nil {
				return Error.Wrap(err)
			}
			return nil
		}
		return Error.Wrap(err)
	}

	return nil
}

// Get gets the peer identity based on the certificate's nodeID
func (certs *certDB) Get(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxPeerID, err := certs.db.Get_PeerIdentity_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if dbxPeerID == nil {
		return nil, Error.New("unknown nodeID :%+v: %+v", nodeID.Bytes(), err)
	}

	peerIdent, err := identity.DecodePeerIdentity(ctx, dbxPeerID.PeerIdentity)
	return peerIdent, Error.Wrap(err)
}

// BatchGet gets the peer idenities based on the certificate's nodeID
func (certs *certDB) BatchGet(ctx context.Context, nodeIDs storj.NodeIDList) (peerIdents []*identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	args := make([]interface{}, 0, nodeIDs.Len())
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
	}

	rows, err := certs.db.Query(certs.db.Rebind(`
			SELECT * FROM peer_identities WHERE node_id IN (?`+strings.Repeat(", ?", len(nodeIDs)-1)+`)`), args...)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		r := &dbx.PeerIdentity{}
		err := rows.Scan(&r.SerialNumber, &r.PeerIdentity, &r.NodeId, &r.UpdateAt)
		if err != nil {
			return peerIdents, Error.Wrap(err)
		}
		peerIdent, err := identity.DecodePeerIdentity(ctx, r.PeerIdentity)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		peerIdents = append(peerIdents, peerIdent)
	}
	return peerIdents, nil
}
