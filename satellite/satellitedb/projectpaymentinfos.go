// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// projectPayments is the an implementation of console.ProjectPayments.
// Allows to work with project payment info storage
type projectPayments struct {
	db      *dbx.DB
	methods dbx.Methods
}

func (pp *projectPayments) Delete(ctx context.Context, projectPaymentID uuid.UUID) error {
	_, err := pp.methods.Delete_ProjectPayment_By_Id(ctx, dbx.ProjectPayment_Id(projectPaymentID[:]))
	return err
}

func (pp *projectPayments) GetByID(ctx context.Context, projectPaymentID uuid.UUID) (*console.ProjectPayment, error) {
	dbxInfo, err := pp.methods.Get_ProjectPayment_By_Id(ctx, dbx.ProjectPayment_Id(projectPaymentID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(ctx, dbxInfo)
}

func (pp *projectPayments) Update(ctx context.Context, info console.ProjectPayment) error {
	updateFields := dbx.ProjectPayment_Update_Fields{
		IsDefault: dbx.ProjectPayment_IsDefault(info.IsDefault),
	}

	_, err := pp.methods.Update_ProjectPayment_By_Id(ctx, dbx.ProjectPayment_Id(info.ID[:]), updateFields)
	return err
}

func (pp *projectPayments) GetDefaultByProjectID(ctx context.Context, projectID uuid.UUID) (*console.ProjectPayment, error) {
	dbxInfo, err := pp.methods.Get_ProjectPayment_By_ProjectId_And_IsDefault_Equal_True(ctx, dbx.ProjectPayment_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(ctx, dbxInfo)
}

// Create stores new project payment info into db
func (pp *projectPayments) Create(ctx context.Context, info console.ProjectPayment) (*console.ProjectPayment, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbxInfo, err := pp.methods.Create_ProjectPayment(ctx,
		dbx.ProjectPayment_Id(id[:]),
		dbx.ProjectPayment_ProjectId(info.ProjectID[:]),
		dbx.ProjectPayment_PayerId(info.PayerID[:]),
		dbx.ProjectPayment_PaymentMethodId(info.PaymentMethodID),
		dbx.ProjectPayment_IsDefault(info.IsDefault))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectPayment(ctx, dbxInfo)
}

// GetByProjectID retrieves project payment info from db by projectID
func (pp *projectPayments) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*console.ProjectPayment, error) {
	dbxInfos, err := pp.methods.All_ProjectPayment_By_ProjectId(ctx, dbx.ProjectPayment_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPaymentSlice(ctx, dbxInfos)
}

// GetByPayerID retrieves project payment info from db by payerID(userID)
func (pp *projectPayments) GetByPayerID(ctx context.Context, payerID uuid.UUID) ([]*console.ProjectPayment, error) {
	dbxInfos, err := pp.methods.All_ProjectPayment_By_PayerId(ctx, dbx.ProjectPayment_PayerId(payerID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXProjectPaymentSlice(ctx, dbxInfos)
}

// fromDBXProjectPayment is a helper method to convert from *dbx.ProjectPayment to *console.ProjectPayment
func fromDBXProjectPayment(ctx context.Context, dbxInfo *dbx.ProjectPayment) (_ *console.ProjectPayment, err error) {
	defer mon.Task()(&ctx)(&err)
	projectID, err := bytesToUUID(dbxInfo.ProjectId)
	if err != nil {
		return nil, err
	}

	payerID, err := bytesToUUID(dbxInfo.PayerId)
	if err != nil {
		return nil, err
	}

	id, err := bytesToUUID(dbxInfo.Id)
	if err != nil {
		return nil, err
	}

	return &console.ProjectPayment{
		ID:              id,
		ProjectID:       projectID,
		PayerID:         payerID,
		PaymentMethodID: dbxInfo.PaymentMethodId,
		CreatedAt:       dbxInfo.CreatedAt,
		IsDefault:       dbxInfo.IsDefault,
	}, nil
}

func fromDBXProjectPaymentSlice(ctx context.Context, dbxInfos []*dbx.ProjectPayment) (_ []*console.ProjectPayment, err error) {
	defer mon.Task()(&ctx)(&err)
	var projectPayments []*console.ProjectPayment
	var errors []error

	// Generating []dbo from []dbx and collecting all errors
	for _, paymentMethodDBX := range dbxInfos {
		projectPayment, err := fromDBXProjectPayment(ctx, paymentMethodDBX)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		projectPayments = append(projectPayments, projectPayment)
	}

	return projectPayments, errs.Combine(errors...)
}
