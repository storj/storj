// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, rba *pb.RenterBandwidthAllocation) error {
	rbaBytes, err := proto.Marshal(rba)
	if err != nil {
		return err
	}
	expiration := time.Unix(rba.PayerAllocation.ExpirationUnixSec, 0)
	_, err = b.db.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Serialnum(rba.PayerAllocation.SerialNumber+rba.StorageNodeId.String()),
		dbx.Bwagreement_Data(rbaBytes),
		dbx.Bwagreement_StorageNode(rba.StorageNodeId.Bytes()),
		dbx.Bwagreement_Action(int64(rba.PayerAllocation.Action)),
		dbx.Bwagreement_Total(rba.Total),
		dbx.Bwagreement_ExpiresAt(expiration),
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
		rba := pb.RenterBandwidthAllocation{}
		err := proto.Unmarshal(entry.Data, &rba)
		if err != nil {
			return nil, err
		}
		agreement := &agreements[i]
		agreement.Agreement = rba
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
		rba := pb.RenterBandwidthAllocation{}
		err := proto.Unmarshal(entry.Data, &rba)
		if err != nil {
			return nil, err
		}
		agreement := &agreements[i]
		agreement.Agreement = rba
		agreement.CreatedAt = entry.CreatedAt
	}
	return agreements, nil
}

func (b *bandwidthagreement) DeletePaidAndExpired(ctx context.Context) error {
	// TODO: implement deletion of paid and expired BWAs
	return Error.New("DeletePaidAndExpired not implemented")
}
