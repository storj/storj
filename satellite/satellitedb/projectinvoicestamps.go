// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type projectinvoicestamps struct {
	db dbx.Methods
}

func (db *projectinvoicestamps) Create(ctx context.Context, stamp console.ProjectInvoiceStamp) (*console.ProjectInvoiceStamp, error) {
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

func (db *projectinvoicestamps) GetByProjectIDStartDate(ctx context.Context, projectID uuid.UUID, startDate time.Time) (*console.ProjectInvoiceStamp, error) {
	dbxStamp, err := db.db.Get_ProjectInvoiceStamp_By_ProjectId_And_StartDate(ctx,
		dbx.ProjectInvoiceStamp_ProjectId(projectID[:]),
		dbx.ProjectInvoiceStamp_StartDate(startDate))

	if err != nil {
		return nil, err
	}

	return fromDBXProjectInvoiceStamp(dbxStamp)
}

func (db *projectinvoicestamps) GetAll(ctx context.Context, projectID uuid.UUID) ([]console.ProjectInvoiceStamp, error) {
	dbxStamps, err := db.db.All_ProjectInvoiceStamp_By_ProjectId_OrderBy_Desc_StartDate(ctx, dbx.ProjectInvoiceStamp_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	var stamps []console.ProjectInvoiceStamp
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
// *console.ProjectInvoiceStamp
func fromDBXProjectInvoiceStamp(dbxStamp *dbx.ProjectInvoiceStamp) (*console.ProjectInvoiceStamp, error) {
	projectID, err := bytesToUUID(dbxStamp.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.ProjectInvoiceStamp{
		ProjectID: projectID,
		InvoiceID: dbxStamp.InvoiceId,
		StartDate: dbxStamp.StartDate,
		EndDate:   dbxStamp.EndDate,
		CreatedAt: dbxStamp.CreatedAt,
	}, nil
}
