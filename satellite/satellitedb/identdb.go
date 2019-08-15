// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type identDB struct {
	db *dbx.DB
}

// Set adds a peer identity entry
func (idents *identDB) Set(ctx context.Context, nodeID storj.NodeID, peerIdent *identity.PeerIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := idents.db.Begin()
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

	var serialNum []byte
	query := `SELECT serial_number FROM peer_identities WHERE node_id = ?;`
	err = tx.QueryRow(idents.db.Rebind(query), nodeID.Bytes()).Scan(&serialNum)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = tx.Exec(idents.db.Rebind(
				`INSERT INTO peer_identities 
				( serial_number, peer_chain, node_id, updated_at ) 
				VALUES ( ?, ?, ?, ? );`),
				peerIdent.Leaf.SerialNumber.Bytes(), chain, nodeID.Bytes(), time.Now())
			if err != nil {
				return Error.Wrap(err)
			}
			return nil
		}
		return Error.Wrap(err)
	}

	if !bytes.Equal(serialNum, peerIdent.Leaf.SerialNumber.Bytes()) {
		_, err = tx.Exec(idents.db.Rebind(
			`UPDATE peer_identities SET 
			node_id = ?, serial_number = ?, 
			peer_chain = ?, updated_at = ? 
			WHERE node_id = ?`),
			nodeID.Bytes(), peerIdent.Leaf.SerialNumber.Bytes(), chain, time.Now(), nodeID.Bytes())
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// Get gets the peer identity based on the certificate's nodeID
func (idents *identDB) Get(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxPeerID, err := idents.db.Get_PeerIdentity_By_NodeId_OrderBy_Desc_UpdatedAt(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if dbxPeerID == nil {
		return nil, Error.New("unknown nodeID :%+v: %+v", nodeID.Bytes(), err)
	}

	peerIdent, err := identity.DecodePeerIdentity(ctx, dbxPeerID.PeerChain)
	return peerIdent, Error.Wrap(err)
}

// BatchGet gets the peer idenities based on the certificate's nodeID
func (idents *identDB) BatchGet(ctx context.Context, nodeIDs storj.NodeIDList) (peerIdents []*identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	args := make([]interface{}, 0, nodeIDs.Len())
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
	}

	rows, err := idents.db.Query(idents.db.Rebind(`
			SELECT peer_chain FROM peer_identities WHERE node_id IN (?`+strings.Repeat(", ?", len(nodeIDs)-1)+`)`), args...)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var peerChain []byte
		err := rows.Scan(&peerChain)
		if err != nil {
			return peerIdents, Error.Wrap(err)
		}
		peerIdent, err := identity.DecodePeerIdentity(ctx, peerChain)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		peerIdents = append(peerIdents, peerIdent)
	}
	return peerIdents, nil
}
