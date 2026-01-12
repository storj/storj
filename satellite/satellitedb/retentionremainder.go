// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// retentionRemainderDB provides access to retention remainder charges.
type retentionRemainderDB struct {
	db *satelliteDB
}

// Upsert inserts a new deletion remainder charge record or updates an existing one by adding the remainder byte hours.
// The upsert hinges on a unique constraint on (project_id, bucket_name, deleted_at). deleted_at is at month precision.
func (d *retentionRemainderDB) Upsert(ctx context.Context, charge accounting.RetentionRemainderCharge) (err error) {
	defer mon.Task()(&ctx)(&err)

	charge.DeletedAt = time.Date(charge.DeletedAt.Year(), charge.DeletedAt.Month(), 1, 0, 0, 0, 0, charge.DeletedAt.Location()).UTC()

	switch d.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		query := `INSERT INTO retention_remainder_charges (project_id, bucket_name, deleted_at, remainder_byte_hours, product_id, billed)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT(project_id, bucket_name, deleted_at)
				DO UPDATE SET remainder_byte_hours = retention_remainder_charges.remainder_byte_hours + EXCLUDED.remainder_byte_hours`

		_, err = d.db.ExecContext(ctx, d.db.Rebind(query), charge.ProjectID, charge.BucketName, charge.DeletedAt, charge.RemainderByteHours, charge.ProductID, charge.Billed)
		return Error.Wrap(err)
	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, d.db, func(client *spanner.Client) (err error) {
			statements := []spanner.Statement{
				{
					SQL: `
						UPDATE retention_remainder_charges
						SET remainder_byte_hours = remainder_byte_hours + @remainder_byte_hours
						WHERE (project_id, bucket_name, deleted_at) = (@project_id, @bucket_name, @deleted_at)
					`,
					Params: map[string]any{
						"remainder_byte_hours": charge.RemainderByteHours,
						"project_id":           charge.ProjectID.Bytes(),
						"bucket_name":          []byte(charge.BucketName),
						"deleted_at":           charge.DeletedAt,
					},
				},
				{
					SQL: `
						INSERT OR IGNORE INTO retention_remainder_charges
							(project_id, bucket_name, deleted_at, remainder_byte_hours, product_id, billed)
						VALUES (@project_id, @bucket_name, @deleted_at, @remainder_byte_hours, @product_id, @billed)
					`,
					Params: map[string]any{
						"project_id":           charge.ProjectID.Bytes(),
						"bucket_name":          []byte(charge.BucketName),
						"deleted_at":           charge.DeletedAt,
						"remainder_byte_hours": charge.RemainderByteHours,
						"product_id":           int64(charge.ProductID),
						"billed":               charge.Billed,
					},
				},
			}

			_, err = client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
				_, err := txn.BatchUpdate(ctx, statements)
				return err
			}, spanner.TransactionOptions{
				TransactionTag: "accounting/insert-retention-remainder-charge",
			})
			return Error.Wrap(err)
		})
	default:
		return Error.New("unsupported database dialect: %s", d.db.impl)
	}
}

// GetUnbilledCharges retrieves unbilled charges for a project in the given time period.
func (d *retentionRemainderDB) GetUnbilledCharges(ctx context.Context, options accounting.GetUnbilledChargesOptions) (_ []accounting.RetentionRemainderCharge, nextToken *accounting.RetentionRemainderContinuationToken, err error) {
	defer mon.Task()(&ctx)(&err)

	if options.Limit <= 0 || options.Limit > maxLimit {
		return nil, nil, Error.New("limit must be between 1 and %d", maxLimit)
	}

	rows, nextToken, err := d.db.Paged_RetentionRemainderChargesInDeletedAtRangeByProjectID(ctx,
		dbx.RetentionRemainderCharge_ProjectId(options.ProjectID[:]),
		dbx.RetentionRemainderCharge_DeletedAt(options.From),
		dbx.RetentionRemainderCharge_DeletedAt(options.To),
		options.Limit,
		options.NextToken,
	)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	charges := make([]accounting.RetentionRemainderCharge, 0, len(rows))
	for _, row := range rows {
		charge, err := fromDBXRetentionRemainderCharge(row)
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		charges = append(charges, charge)
	}

	return charges, nextToken, nil
}

// MarkChargesAsBilled marks all unbilled charges in the time period as billed.
func (d *retentionRemainderDB) MarkChargesAsBilled(ctx context.Context, projectID uuid.UUID, from, to time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = d.db.ExecContext(ctx, d.db.Rebind(`
		UPDATE retention_remainder_charges
		SET billed = true
		WHERE project_id = ?
		  AND deleted_at >= ?
		  AND deleted_at < ?
		  AND billed = false
	`), projectID, from, to)

	return Error.Wrap(err)
}

// fromDBXRetentionRemainderCharge converts a DBX deletion remainder charge to the accounting type.
func fromDBXRetentionRemainderCharge(row *dbx.RetentionRemainderCharge) (accounting.RetentionRemainderCharge, error) {
	projectID, err := uuid.FromBytes(row.ProjectId)
	if err != nil {
		return accounting.RetentionRemainderCharge{}, errs.Wrap(err)
	}

	var productID int32
	if row.ProductId != nil {
		productID = int32(*row.ProductId)
	}

	return accounting.RetentionRemainderCharge{
		ProjectID:          projectID,
		BucketName:         string(row.BucketName),
		DeletedAt:          row.DeletedAt,
		RemainderByteHours: float64(row.RemainderByteHours),
		ProductID:          productID,
		Billed:             row.Billed,
	}, nil
}
