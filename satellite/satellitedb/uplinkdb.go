// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/uplinkdb"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type uplinkDB struct {
	db *dbx.DB
}

func (b *uplinkDB) SavePublicKey(ctx context.Context, agreement uplinkdb.Agreement) error {
	tx, err := b.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Get_CertDB_By_Id(ctx, dbx.CertDB_Id(agreement.ID))
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = tx.Create_CertDB(
			ctx,
			dbx.CertDB_Publickey(agreement.PublicKey),
			dbx.CertDB_Id(agreement.ID),
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

func (b *uplinkDB) GetPublicKey(ctx context.Context, nodeID []byte) (*uplinkdb.Agreement, error) {
	dbxInfo, err := b.db.Get_CertDB_By_Id(ctx, dbx.CertDB_Id(nodeID))
	if err != nil {
		return &uplinkdb.Agreement{}, err
	}

	return &uplinkdb.Agreement{
		ID:        dbxInfo.Id,
		PublicKey: dbxInfo.Publickey,
	}, nil
}
