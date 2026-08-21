// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that invoiceLicenseRecords implements stripe.LicenseRecordsDB.
var _ stripe.LicenseRecordsDB = (*invoiceLicenseRecords)(nil)

type invoiceLicenseRecordState int

const (
	// invoice license record is not yet applied to customer invoice.
	invoiceLicenseRecordStateUnapplied invoiceLicenseRecordState = 0
	// invoice license record has been used during creating customer invoice.
	invoiceLicenseRecordStateConsumed invoiceLicenseRecordState = 1
)

// Int returns the state as int.
func (state invoiceLicenseRecordState) Int() int {
	return int(state)
}

type invoiceLicenseRecords struct {
	db *satelliteDB
}

// Create creates new invoice license records in the DB. The records must be
// distinct by user, product and billed days, since that is the identity of a
// charge; a batch containing two records for the same one is rejected with
// ErrDuplicateLicenseRecord rather than written in part. Callers aggregate seats
// per rate before recording them.
func (db *invoiceLicenseRecords) Create(ctx context.Context, records []stripe.CreateLicenseRecord, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	type key struct {
		userID     uuid.UUID
		productID  int32
		billedDays int
	}
	seen := make(map[key]struct{}, len(records))
	for _, record := range records {
		k := key{userID: record.UserID, productID: record.ProductID, billedDays: record.BilledDays}
		if _, ok := seen[k]; ok {
			return Error.New("%w: user %s, product %d, %d billed days",
				stripe.ErrDuplicateLicenseRecord, record.UserID, record.ProductID, record.BilledDays)
		}
		seen[k] = struct{}{}
	}

	return Error.Wrap(db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, record := range records {
			id, err := uuid.New()
			if err != nil {
				return Error.Wrap(err)
			}

			_, err = tx.Create_StripecoinpaymentsInvoiceLicenseRecord(ctx,
				dbx.StripecoinpaymentsInvoiceLicenseRecord_Id(id[:]),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_UserId(record.UserID[:]),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_ProductId(int(record.ProductID)),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_Seats(record.Seats),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_BilledDays(record.BilledDays),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_DaysInPeriod(record.DaysInPeriod),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_UnitAmountCents(record.UnitAmountCents),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodStart(start),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodEnd(end),
				dbx.StripecoinpaymentsInvoiceLicenseRecord_State(invoiceLicenseRecordStateUnapplied.Int()),
			)
			if err != nil {
				return Error.Wrap(err)
			}
		}

		return nil
	}))
}

// Check checks if an invoice license record for the specified user, product, prorated
// rate and billing period exists.
func (db *invoiceLicenseRecords) Check(ctx context.Context, userID uuid.UUID, productID int32, billedDays int, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Get_StripecoinpaymentsInvoiceLicenseRecord_By_UserId_And_ProductId_And_BilledDays_And_PeriodStart_And_PeriodEnd(ctx,
		dbx.StripecoinpaymentsInvoiceLicenseRecord_UserId(userID[:]),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_ProductId(int(productID)),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_BilledDays(billedDays),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodStart(start),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodEnd(end),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return Error.Wrap(err)
	}

	return stripe.ErrLicenseRecordExists
}

// ListUnappliedByUser returns the records for a user and billing period that have not
// been turned into invoice items yet.
func (db *invoiceLicenseRecords) ListUnappliedByUser(ctx context.Context, userID uuid.UUID, start, end time.Time) (_ []stripe.LicenseRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRecords, err := db.db.All_StripecoinpaymentsInvoiceLicenseRecord_By_UserId_And_PeriodStart_And_PeriodEnd_And_State(ctx,
		dbx.StripecoinpaymentsInvoiceLicenseRecord_UserId(userID[:]),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodStart(start),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_PeriodEnd(end),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_State(invoiceLicenseRecordStateUnapplied.Int()),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	records := make([]stripe.LicenseRecord, 0, len(dbxRecords))
	for _, dbxRecord := range dbxRecords {
		record, err := fromDBXInvoiceLicenseRecord(dbxRecord)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		records = append(records, *record)
	}

	return records, nil
}

// Consume marks an invoice license record as applied.
func (db *invoiceLicenseRecords) Consume(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_StripecoinpaymentsInvoiceLicenseRecord_By_Id(ctx,
		dbx.StripecoinpaymentsInvoiceLicenseRecord_Id(id[:]),
		dbx.StripecoinpaymentsInvoiceLicenseRecord_Update_Fields{
			State: dbx.StripecoinpaymentsInvoiceLicenseRecord_State(invoiceLicenseRecordStateConsumed.Int()),
		},
	)

	return Error.Wrap(err)
}

// fromDBXInvoiceLicenseRecord converts *dbx.StripecoinpaymentsInvoiceLicenseRecord to *stripe.LicenseRecord.
func fromDBXInvoiceLicenseRecord(dbxRecord *dbx.StripecoinpaymentsInvoiceLicenseRecord) (*stripe.LicenseRecord, error) {
	id, err := uuid.FromBytes(dbxRecord.Id)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	userID, err := uuid.FromBytes(dbxRecord.UserId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &stripe.LicenseRecord{
		ID:              id,
		UserID:          userID,
		ProductID:       int32(dbxRecord.ProductId),
		Seats:           dbxRecord.Seats,
		BilledDays:      dbxRecord.BilledDays,
		DaysInPeriod:    dbxRecord.DaysInPeriod,
		UnitAmountCents: dbxRecord.UnitAmountCents,
		PeriodStart:     dbxRecord.PeriodStart,
		PeriodEnd:       dbxRecord.PeriodEnd,
		State:           dbxRecord.State,
	}, nil
}
