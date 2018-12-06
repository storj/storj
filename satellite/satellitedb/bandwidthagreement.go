// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/storj/pkg/bwagreement"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, agreement bwagreement.Agreement) error {
	_, err := b.db.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(agreement.Signature),
		dbx.Bwagreement_Data(agreement.Agreement),
	)
	return err
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
