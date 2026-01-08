// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
)

// deletionRemainderDB provides access to deletion remainder charges.
type deletionRemainderDB struct {
	db *satelliteDB
}

// Create inserts a new deletion remainder charge record.
func (d *deletionRemainderDB) Create(ctx context.Context, charge accounting.DeletionRemainderCharge) (err error) {
	defer mon.Task()(&ctx)(&err)

	optional := dbx.DeletionRemainderCharge_Create_Fields{
		ProductId: dbx.DeletionRemainderCharge_ProductId(int(charge.ProductID)),
		Billed:    dbx.DeletionRemainderCharge_Billed(charge.Billed),
	}

	return d.db.CreateNoReturn_DeletionRemainderCharge(ctx,
		dbx.DeletionRemainderCharge_ProjectId(charge.ProjectID[:]),
		dbx.DeletionRemainderCharge_BucketName([]byte(charge.BucketName)),
		dbx.DeletionRemainderCharge_CreatedAt(charge.CreatedAt),
		dbx.DeletionRemainderCharge_DeletedAt(charge.DeletedAt),
		dbx.DeletionRemainderCharge_ObjectSize(charge.ObjectSize),
		dbx.DeletionRemainderCharge_RemainderHours(float32(charge.RemainderHours)),
		optional,
	)
}

// GetUnbilledCharges retrieves all unbilled charges for a project in the given time period.
func (d *deletionRemainderDB) GetUnbilledCharges(ctx context.Context, projectID uuid.UUID, from, to time.Time) (_ []accounting.DeletionRemainderCharge, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := d.db.All_DeletionRemainderCharge_By_ProjectId_And_DeletedAt_GreaterOrEqual_And_DeletedAt_Less_And_Billed_Equal_False(
		ctx,
		dbx.DeletionRemainderCharge_ProjectId(projectID[:]),
		dbx.DeletionRemainderCharge_DeletedAt(from),
		dbx.DeletionRemainderCharge_DeletedAt(to),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	charges := make([]accounting.DeletionRemainderCharge, 0, len(rows))
	for _, row := range rows {
		charge, err := fromDBXDeletionRemainderCharge(row)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		charges = append(charges, charge)
	}

	return charges, nil
}

// MarkChargesAsBilled marks all unbilled charges in the time period as billed.
func (d *deletionRemainderDB) MarkChargesAsBilled(ctx context.Context, projectID uuid.UUID, from, to time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch d.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		_, err = d.db.ExecContext(ctx, `
		UPDATE deletion_remainder_charges
		SET billed = true
		WHERE project_id = $1
		  AND deleted_at >= $2
		  AND deleted_at < $3
		  AND billed = false
	`, projectID[:], from, to)

	case dbutil.Spanner:
		_, err = d.db.ExecContext(ctx, `
		UPDATE deletion_remainder_charges
		SET billed = true
		WHERE project_id = @project_id
		  AND deleted_at >= @deleted_at_from
		  AND deleted_at < @deleted_at_to
		  AND billed = false
	`, sql.Named("project_id", projectID.Bytes()), sql.Named("deleted_at_from", from), sql.Named("deleted_at_to", to))

	default:
		return Error.New("unsupported database implementation: %v", d.db.impl)
	}

	return Error.Wrap(err)
}

// fromDBXDeletionRemainderCharge converts a DBX deletion remainder charge to the accounting type.
func fromDBXDeletionRemainderCharge(row *dbx.DeletionRemainderCharge) (accounting.DeletionRemainderCharge, error) {
	projectID, err := uuid.FromBytes(row.ProjectId)
	if err != nil {
		return accounting.DeletionRemainderCharge{}, errs.Wrap(err)
	}

	var productID int32
	if row.ProductId != nil {
		productID = int32(*row.ProductId)
	}

	return accounting.DeletionRemainderCharge{
		ProjectID:      projectID,
		BucketName:     string(row.BucketName),
		CreatedAt:      row.CreatedAt,
		DeletedAt:      row.DeletedAt,
		ObjectSize:     row.ObjectSize,
		RemainderHours: float64(row.RemainderHours),
		ProductID:      productID,
		Billed:         row.Billed,
	}, nil
}
