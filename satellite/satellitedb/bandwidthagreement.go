// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/bwagreement"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, agreement *bwagreement.Agreement) error {
	_, err := b.db.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(agreement.Signature),
		dbx.Bwagreement_Data(agreement.Agreement),
	)
	return err
}

func (b *bandwidthagreement) GetAllAgreements(ctx context.Context) ([]*bwagreement.Agreement, error) {
	rows, err := b.db.All_Bwagreement(ctx)
	if err != nil {
		return nil, err
	}

	var agreements []*bwagreement.Agreement
	for _, entry := range rows {
		agreements = append(agreements, &bwagreement.Agreement{
			Signature: entry.Signature,
			Agreement: entry.Data,
		})
	}
	return agreements, nil
}
