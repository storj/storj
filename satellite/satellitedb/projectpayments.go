// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/stripepayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// projectpayments is the an implementation of stripepayments.ProjectPayments.
// Allows to work with project payment info storage
type projectpayments struct {
	db dbx.Methods
}

// Create stores new project payment info into db
func (infos *projectpayments) Create(ctx context.Context, info stripepayments.ProjectPayment) (*stripepayments.ProjectPayment, error) {
	dbxInfo, err := infos.db.Create_ProjectPayment(ctx,
		dbx.ProjectPayment_ProjectId(info.ProjectID[:]),
		dbx.ProjectPayment_PayerId(info.PayerID[:]),
		dbx.ProjectPayment_PaymentMethodId(info.PaymentMethodID))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(dbxInfo)
}

// GetByProjectID retrieves project payment info from db by projectID
func (infos *projectpayments) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*stripepayments.ProjectPayment, error) {
	dbxInfo, err := infos.db.Get_ProjectPayment_By_ProjectId(ctx, dbx.ProjectPayment_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(dbxInfo)
}

// GetByPayerID retrieves project payment info from db by payerID(userID)
func (infos *projectpayments) GetByPayerID(ctx context.Context, payerID uuid.UUID) (*stripepayments.ProjectPayment, error) {
	dbxInfo, err := infos.db.Get_ProjectPayment_By_PayerId(ctx, dbx.ProjectPayment_PayerId(payerID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(dbxInfo)
}

// fromDBXProjectPayment is a helper method to convert from *dbx.ProjectPayment to *stripepayments.ProjectPayment
func fromDBXProjectPayment(dbxInfo *dbx.ProjectPayment) (*stripepayments.ProjectPayment, error) {
	projectID, err := bytesToUUID(dbxInfo.ProjectId)
	if err != nil {
		return nil, err
	}

	payerID, err := bytesToUUID(dbxInfo.PayerId)
	if err != nil {
		return nil, err
	}

	return &stripepayments.ProjectPayment{
		ProjectID:       projectID,
		PayerID:         payerID,
		PaymentMethodID: dbxInfo.PaymentMethodId,
		CreatedAt:       dbxInfo.CreatedAt,
	}, nil
}
