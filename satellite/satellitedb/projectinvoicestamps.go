// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/stripepayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// projectInvoiceStamps is the an implementation of stripepayments.ProjectInvoiceStamps.
// Allows to work with project invoice stamps storage
type projectInvoiceStamps struct {
	db dbx.Methods
}

// Create stores new project invoice stamp into db
func (db *projectInvoiceStamps) Create(ctx context.Context, stamp stripepayments.ProjectInvoiceStamp) (*stripepayments.ProjectInvoiceStamp, error) {
	dbxStamp, err := db.db.Create_ProjectInvoiceStamp(ctx,
		dbx.ProjectInvoiceStamp_ProjectId(stamp.ProjectID[:]),
		dbx.ProjectInvoiceStamp_InvoiceId(stamp.InvoiceID),
		dbx.ProjectInvoiceStamp_StartDate(stamp.StartDate),
		dbx.ProjectInvoiceStamp_EndDate(stamp.EndDate),
		dbx.ProjectInvoiceStamp_CreatedAt(stamp.CreatedAt))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectInvoiceStamp(dbxStamp)
}

// GetByProjectIDStartDate retrieves project invoice id by projectID and start date
func (db *projectInvoiceStamps) GetByProjectIDStartDate(ctx context.Context, projectID uuid.UUID, startDate time.Time) (*stripepayments.ProjectInvoiceStamp, error) {
	dbxStamp, err := db.db.Get_ProjectInvoiceStamp_By_ProjectId_And_StartDate(ctx,
		dbx.ProjectInvoiceStamp_ProjectId(projectID[:]),
		dbx.ProjectInvoiceStamp_StartDate(startDate))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectInvoiceStamp(dbxStamp)
}

// GetAll retrieves all project invoice stamps for particular project
func (db *projectInvoiceStamps) GetAll(ctx context.Context, projectID uuid.UUID) ([]stripepayments.ProjectInvoiceStamp, error) {
	dbxStamps, err := db.db.All_ProjectInvoiceStamp_By_ProjectId_OrderBy_Desc_StartDate(ctx, dbx.ProjectInvoiceStamp_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	var stamps []stripepayments.ProjectInvoiceStamp
	for _, dbxStamp := range dbxStamps {
		stamp, err := fromDBXProjectInvoiceStamp(dbxStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, *stamp)
	}

	return stamps, nil
}

// fromDBXProjectInvoiceStamp helper function to conert *dbx.ProjectInvoiceStamp to
// *stripepayments.ProjectInvoiceStamp
func fromDBXProjectInvoiceStamp(dbxStamp *dbx.ProjectInvoiceStamp) (*stripepayments.ProjectInvoiceStamp, error) {
	projectID, err := bytesToUUID(dbxStamp.ProjectId)
	if err != nil {
		return nil, err
	}

	return &stripepayments.ProjectInvoiceStamp{
		ProjectID: projectID,
		InvoiceID: dbxStamp.InvoiceId,
		StartDate: dbxStamp.StartDate,
		EndDate:   dbxStamp.EndDate,
		CreatedAt: dbxStamp.CreatedAt,
	}, nil
}
