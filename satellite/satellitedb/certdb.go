// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

func (certs *certDB) SavePublicKey(ctx context.Context, nodeID storj.NodeID, publicKey crypto.PublicKey) (err error) {
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

	pubbytes, err := pkcrypto.PublicKeyToPKIX(publicKey)
	if err != nil {
		return Error.Wrap(err)
	}

	var node []byte
	query := `SELECT node_id FROM certRecords WHERE publickey = ?;`
	err = tx.QueryRow(certs.db.Rebind(query), pubbytes).Scan(&node)
	if err != nil {
		if err == sql.ErrNoRows {
			// create a new entry
			_, err = tx.Exec(certs.db.Rebind(`INSERT INTO certRecords ( publickey, node_id, update_at ) VALUES ( ?, ?, ? );`), pubbytes, nodeID.Bytes(), time.Now())
			if err != nil {
				return Error.Wrap(err)
			}
			return nil
		}
		return Error.Wrap(err)
	}

	return nil
}

// GetPublicKey gets the public key of uplink corresponding to uplink id
func (certs *certDB) GetPublicKey(ctx context.Context, nodeID storj.NodeID) (_ crypto.PublicKey, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := certs.db.All_CertRecord_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.CertRecord_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	if len(dbxInfo) == 0 {
		return nil, Error.New("Invalid nodeID : %+v: %+v ", nodeID.String(), err)
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
	dbxInfo, err := certs.db.All_CertRecord_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.CertRecord_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	if len(dbxInfo) == 0 {
		return nil, Error.New("Invalid nodeID : %+v: %+v ", nodeID.String(), err)
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
