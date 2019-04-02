// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

func (b *certDB) SavePublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) error {
	tx, err := b.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Get_CertRecord_By_Id(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		// no rows err, so create/insert an entry
		pubbytes, err := pkcrypto.PublicKeyToPKIX(publicKey)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
		_, err = tx.Create_CertRecord(ctx,
			dbx.CertRecord_Publickey(pubbytes),
			dbx.CertRecord_Id(nodeID.Bytes()),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		// nodeID entry already exists, just return
		return Error.Wrap(tx.Rollback())
	}

	return Error.Wrap(tx.Commit())
}

func (b *certDB) GetPublicKey(ctx context.Context, nodeID storj.NodeID) (crypto.PublicKey, error) {
	dbxInfo, err := b.db.Get_CertRecord_By_Id(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}
	pubkey, err := pkcrypto.PublicKeyFromPKIX(dbxInfo.Publickey)
	if err != nil {
		return nil, Error.New("Failed to extract Public Key from Order: %+v", err)
	}
	return pubkey, nil
}
