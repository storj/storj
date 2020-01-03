// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type peerIdentities struct {
	db *satelliteDB
}

// Set adds a peer identity entry
func (idents *peerIdentities) Set(ctx context.Context, nodeID storj.NodeID, ident *identity.PeerIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	if ident == nil {
		return Error.New("identitiy is nil")
	}

	tx, err := idents.db.Open(ctx)
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

	serial, err := tx.Get_PeerIdentity_LeafSerialNumber_By_NodeId(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
	if serial == nil || err != nil {
		if serial == nil || err == sql.ErrNoRows {
			return Error.Wrap(tx.CreateNoReturn_PeerIdentity(ctx,
				dbx.PeerIdentity_NodeId(nodeID.Bytes()),
				dbx.PeerIdentity_LeafSerialNumber(ident.Leaf.SerialNumber.Bytes()),
				dbx.PeerIdentity_Chain(identity.EncodePeerIdentity(ident)),
			))
		}
		return Error.Wrap(err)
	}
	if !bytes.Equal(serial.LeafSerialNumber, ident.Leaf.SerialNumber.Bytes()) {
		return Error.Wrap(tx.UpdateNoReturn_PeerIdentity_By_NodeId(ctx,
			dbx.PeerIdentity_NodeId(nodeID.Bytes()),
			dbx.PeerIdentity_Update_Fields{
				LeafSerialNumber: dbx.PeerIdentity_LeafSerialNumber(ident.Leaf.SerialNumber.Bytes()),
				Chain:            dbx.PeerIdentity_Chain(identity.EncodePeerIdentity(ident)),
			},
		))
	}

	return Error.Wrap(err)
}

// Get gets the peer identity based on the certificate's nodeID
func (idents *peerIdentities) Get(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxIdent, err := idents.db.Get_PeerIdentity_By_NodeId(ctx, dbx.PeerIdentity_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if dbxIdent == nil {
		return nil, Error.New("missing node id: %v", nodeID)
	}

	ident, err := identity.DecodePeerIdentity(ctx, dbxIdent.Chain)
	return ident, Error.Wrap(err)
}

// BatchGet gets the peer idenities based on the certificate's nodeID
func (idents *peerIdentities) BatchGet(ctx context.Context, nodeIDs storj.NodeIDList) (peerIdents []*identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	args := make([]interface{}, 0, nodeIDs.Len())
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
	}

	// TODO: optimize using arrays like overlay

	rows, err := idents.db.Query(idents.db.Rebind(`
			SELECT chain FROM peer_identities WHERE node_id IN (?`+strings.Repeat(", ?", len(nodeIDs)-1)+`)`), args...)
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
			return nil, Error.Wrap(err)
		}
		ident, err := identity.DecodePeerIdentity(ctx, peerChain)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		peerIdents = append(peerIdents, ident)
	}
	return peerIdents, nil
}
