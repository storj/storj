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

func (certs *certDB) SavePublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) (err error) {
	defer mon.Task()(&ctx)(&err)
	pubbytes, err := pkcrypto.PublicKeyToPKIX(publicKey)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = certs.db.Get_CertRecord_By_Publickey(ctx, dbx.CertRecord_Publickey(pubbytes))
	if err != nil {
		if err == sql.ErrNoRows {
			return certs.tryAddPublicKey(ctx, nodeID, publicKey)
		}
		return Error.Wrap(err)
	}

	// publickey for the nodeID entry already exists, just return
	return nil
}

func (certs *certDB) tryAddPublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) (err error) {
	defer mon.Task()(&ctx)(&err)
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

// GetPublicKey gets the public key of uplink corresponding to uplink id
func (certs *certDB) GetPublicKey(ctx context.Context, nodeID storj.NodeID) (_ crypto.PublicKey, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := certs.db.All_CertRecord_By_Id_OrderBy_Desc_UpdateAt(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	// the first indext always holds the lastest of the keys
	pubkey, err := pkcrypto.PublicKeyFromPKIX(dbxInfo[0].Publickey)
	if err != nil {
		return nil, Error.New("Failed to extract Public Key from Order: %+v", err)
	}
	return pubkey, nil
}

// GetPublicKeys gets the public keys of a storagenode corresponding to storagenode id
func (certs *certDB) GetPublicKeys(ctx context.Context, nodeID storj.NodeID) (pubkeys []crypto.PublicKey, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := certs.db.All_CertRecord_By_Id_OrderBy_Desc_UpdateAt(ctx, dbx.CertRecord_Id(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	if len(dbxInfo) == 0 {
		return nil, Error.New("Failed to extract Public Key from ID: %+v", nodeID.String())
	}

	for _, v := range dbxInfo {
		pubkey, err := pkcrypto.PublicKeyFromPKIX(v.Publickey)
		if err != nil {
			return nil, Error.New("Failed to extract Public Key from Order: %+v", err)
		}
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys, nil
}
