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

func (b *uplinkDB) CreateAgreement(ctx context.Context, agreement uplinkdb.Agreement) error {
	// return nil
	tx, err := b.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Get_UplinkDB_By_Id(ctx, dbx.UplinkDB_Id(agreement.ID))
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = tx.Create_UplinkDB(
			ctx,
			dbx.UplinkDB_Publickey(agreement.PublicKey),
			dbx.UplinkDB_Id(agreement.ID),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		// nodeID entry already exists, return err
		return Error.Wrap(utils.CombineErrors(Error.New("NodeID already exists"), tx.Rollback()))
	}

	return Error.Wrap(tx.Commit())
}

func (b *uplinkDB) GetPublicKey(ctx context.Context, nodeID []byte) (*uplinkdb.Agreement, error) {
	dbxInfo, err := b.db.Get_UplinkDB_By_Id(ctx, dbx.UplinkDB_Id(nodeID))
	if err != nil {
		return &uplinkdb.Agreement{}, err
	}

	return &uplinkdb.Agreement{
		ID:        dbxInfo.Id,
		PublicKey: dbxInfo.Publickey,
	}, nil
}
