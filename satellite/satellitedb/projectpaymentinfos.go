// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// projectpaymentinfos is the an implementation of console.ProjectPaymentInfos.
// Allows to work with project payment info storage
type projectpaymentinfos struct {
	db dbx.Methods
}

// Create stores new project payment info into db
func (infos *projectpaymentinfos) Create(ctx context.Context, info console.ProjectPaymentInfo) (*console.ProjectPaymentInfo, error) {
	dbxInfo, err := infos.db.Create_ProjectPaymentInfo(ctx,
		dbx.ProjectPaymentInfo_ProjectId(info.ProjectID[:]),
		dbx.ProjectPaymentInfo_PayerId(info.PayerID[:]),
		dbx.ProjectPaymentInfo_PaymentMethodId(info.PaymentMethodID))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectPaymentInfo(dbxInfo)
}

// GetByProjectID retrieves project payment info from db by projectID
func (infos *projectpaymentinfos) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*console.ProjectPaymentInfo, error) {
	dbxInfo, err := infos.db.Get_ProjectPaymentInfo_By_ProjectId(ctx, dbx.ProjectPaymentInfo_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPaymentInfo(dbxInfo)
}

// GetByPayerID retrieves project payment info from db by payerID(userID)
func (infos *projectpaymentinfos) GetByPayerID(ctx context.Context, payerID uuid.UUID) (*console.ProjectPaymentInfo, error) {
	dbxInfo, err := infos.db.Get_ProjectPaymentInfo_By_PayerId(ctx, dbx.ProjectPaymentInfo_PayerId(payerID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPaymentInfo(dbxInfo)
}

// fromDBXProjectPaymentInfo is a helper method to convert from *dbx.ProjectPaymentInfo to *console.ProjectPaymentInfo
func fromDBXProjectPaymentInfo(dbxInfo *dbx.ProjectPaymentInfo) (*console.ProjectPaymentInfo, error) {
	projectID, err := bytesToUUID(dbxInfo.ProjectId)
	if err != nil {
		return nil, err
	}

	payerID, err := bytesToUUID(dbxInfo.PayerId)
	if err != nil {
		return nil, err
	}

	return &console.ProjectPaymentInfo{
		ProjectID:       projectID,
		PayerID:         payerID,
		PaymentMethodID: dbxInfo.PaymentMethodId,
		CreatedAt:       dbxInfo.CreatedAt,
	}, nil
}
