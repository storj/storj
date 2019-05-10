// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto"
	"database/sql"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

func (certs *certDB) SavePublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) error {
	_, err := certs.db.Get_CertRecord_By_Id(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err == sql.ErrNoRows {
		return certs.tryAddPublicKey(ctx, nodeID, publicKey)
	}
	if err != nil {
		return Error.Wrap(err)
	}

	// nodeID entry already exists, just return
	return nil
}

func (certs *certDB) tryAddPublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) error {
	// no rows err, so create/insert an entry
	pubbytes, err := pkcrypto.PublicKeyToPKIX(publicKey)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: use upsert here instead of create
	_, err = certs.db.Create_CertRecord(ctx,
		dbx.CertRecord_Publickey(pubbytes),
		dbx.CertRecord_Id(nodeID.Bytes()),
	)
	// another goroutine might race to create the cert record, let's ignore that error
	if pgutil.IsConstraintError(err) || sqliteutil.IsConstraintError(err) {
		return nil
	} else if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (certs *certDB) GetPublicKey(ctx context.Context, nodeID storj.NodeID) (crypto.PublicKey, error) {
	dbxInfo, err := certs.db.Get_CertRecord_By_Id(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	pubkey, err := pkcrypto.PublicKeyFromPKIX(dbxInfo.Publickey)
	if err != nil {
		return nil, Error.New("Failed to extract Public Key from Order: %+v", err)
	}
	return pubkey, nil
}
