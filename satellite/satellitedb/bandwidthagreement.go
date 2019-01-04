// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, serialNum string, agreement bwagreement.Agreement) error {
	tx, err := b.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Get_Bwagreement_By_Serialnum(ctx, dbx.Bwagreement_Serialnum(serialNum))
	if err != nil {
		// no rows err, ie no dulicate serialnum check, so create an entry
		_, err = b.db.Create_Bwagreement(
			ctx,
			dbx.Bwagreement_Signature(agreement.Signature),
			dbx.Bwagreement_Serialnum(serialNum),
			dbx.Bwagreement_Data(agreement.Agreement),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
		return Error.Wrap(tx.Commit())
	}
	return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
}

func (b *bandwidthagreement) GetAgreements(ctx context.Context) ([]bwagreement.Agreement, error) {
	rows, err := b.db.All_Bwagreement(ctx)
	if err != nil {
		return nil, err
	}

	agreements := make([]bwagreement.Agreement, len(rows))
	for i, entry := range rows {
		agreement := &agreements[i]
		agreement.Signature = entry.Signature
		agreement.Agreement = entry.Data
		agreement.CreatedAt = entry.CreatedAt
	}
	return agreements, nil
}

func (b *bandwidthagreement) GetAgreementsSince(ctx context.Context, since time.Time) ([]bwagreement.Agreement, error) {
	rows, err := b.db.All_Bwagreement_By_CreatedAt_Greater(ctx, dbx.Bwagreement_CreatedAt(since))
	if err != nil {
		return nil, err
	}

	agreements := make([]bwagreement.Agreement, len(rows))
	for i, entry := range rows {
		agreement := &agreements[i]
		agreement.Signature = entry.Signature
		agreement.Agreement = entry.Data
		agreement.CreatedAt = entry.CreatedAt
	}
	return agreements, nil
}
