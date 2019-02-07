// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
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
		publicKeyEcdsa, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return Error.Wrap(utils.CombineErrors(errs.New("Uplink Private Key is not a valid *ecdsa.PrivateKey"), tx.Rollback()))
		}

		pubbytes, err := x509.MarshalPKIXPublicKey(publicKeyEcdsa)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
		_, err = tx.Create_CertRecord(ctx,
			dbx.CertRecord_Publickey(pubbytes),
			dbx.CertRecord_Id(nodeID.Bytes()),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		// nodeID entry already exists, just return
		return Error.Wrap(tx.Rollback())
	}

	return Error.Wrap(tx.Commit())
}

func (b *certDB) GetPublicKey(ctx context.Context, nodeID storj.NodeID) (*ecdsa.PublicKey, error) {
	dbxInfo, err := b.db.Get_CertRecord_By_Id(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	pubkey, err := x509.ParsePKIXPublicKey(dbxInfo.Publickey)
	if err != nil {
		return nil, Error.Wrap(Error.New("Failed to extract Public Key from RenterBandwidthAllocation: %+v", err))
	}

	// Typecast public key
	pkey, ok := pubkey.(*ecdsa.PublicKey)
	if !ok {
		return nil, Error.Wrap(Error.New("UnsupportedKey error: %+v", pubkey))
	}

	return pkey, nil
}
