// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

// ensure that invoiceProjectRecords implements stripecoinpayments.ProjectRecordsDB.
var _ stripe.ProjectRecordsDB = (*invoiceProjectRecords)(nil)

// invoiceProjectRecordState defines states of the invoice project record.
type invoiceProjectRecordState int

const (
	// invoice project record is not yet applied to customer invoice.
	invoiceProjectRecordStateUnapplied invoiceProjectRecordState = 0
	// invoice project record has been used during creating customer invoice.
	invoiceProjectRecordStateConsumed invoiceProjectRecordState = 1
)

// Int returns intent state as int.
func (intent invoiceProjectRecordState) Int() int {
	return int(intent)
}

// invoiceProjectRecords is stripecoinpayments project records DB.
//
// architecture: Database
type invoiceProjectRecords struct {
	db *satelliteDB
}

// Create creates new invoice project record in the DB.
func (db *invoiceProjectRecords) Create(ctx context.Context, records []stripe.CreateProjectRecord, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	return db.createWithState(ctx, records, invoiceProjectRecordStateUnapplied, start, end)
}

func (db *invoiceProjectRecords) createWithState(ctx context.Context, records []stripe.CreateProjectRecord, state invoiceProjectRecordState, start, end time.Time) error {
	return Error.Wrap(db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for _, record := range records {
			id, err := uuid.New()
			if err != nil {
				return Error.Wrap(err)
			}

			_, err = tx.Create_StripecoinpaymentsInvoiceProjectRecord(ctx,
				dbx.StripecoinpaymentsInvoiceProjectRecord_Id(id[:]),
				dbx.StripecoinpaymentsInvoiceProjectRecord_ProjectId(record.ProjectID[:]),
				dbx.StripecoinpaymentsInvoiceProjectRecord_Storage(record.Storage),
				dbx.StripecoinpaymentsInvoiceProjectRecord_Egress(record.Egress),
				dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodStart(start),
				dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodEnd(end),
				dbx.StripecoinpaymentsInvoiceProjectRecord_State(state.Int()),
				dbx.StripecoinpaymentsInvoiceProjectRecord_Create_Fields{
					Segments: dbx.StripecoinpaymentsInvoiceProjectRecord_Segments(int64(record.Segments)),
				},
			)
			if err != nil {
				return Error.Wrap(err)
			}
		}

		return nil
	}))
}

// Check checks if invoice project record for specified project and billing period exists.
func (db *invoiceProjectRecords) Check(ctx context.Context, projectID uuid.UUID, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Get_StripecoinpaymentsInvoiceProjectRecord_By_ProjectId_And_PeriodStart_And_PeriodEnd(ctx,
		dbx.StripecoinpaymentsInvoiceProjectRecord_ProjectId(projectID[:]),
		dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodStart(start),
		dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodEnd(end),
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return err
	}

	return stripe.ErrProjectRecordExists
}

// Get returns record for specified project and billing period.
func (db *invoiceProjectRecords) Get(ctx context.Context, projectID uuid.UUID, start, end time.Time) (record *stripe.ProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRecord, err := db.db.Get_StripecoinpaymentsInvoiceProjectRecord_By_ProjectId_And_PeriodStart_And_PeriodEnd(ctx,
		dbx.StripecoinpaymentsInvoiceProjectRecord_ProjectId(projectID[:]),
		dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodStart(start),
		dbx.StripecoinpaymentsInvoiceProjectRecord_PeriodEnd(end),
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return fromDBXInvoiceProjectRecord(dbxRecord)
}

// GetUnappliedByProjectIDs returns unapplied records within the billing period pertaining to a list of project IDs.
func (db *invoiceProjectRecords) GetUnappliedByProjectIDs(ctx context.Context, projectIDs []uuid.UUID, start, end time.Time) (records []stripe.ProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	var rows tagsql.Rows

	switch db.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query = db.db.Rebind(`SELECT
					id, project_id, storage, egress, segments, period_start, period_end, state
			FROM
					stripecoinpayments_invoice_project_records
			WHERE
					project_id IN ( SELECT unnest(?::bytea[]))
					AND period_start = ? AND period_end = ? AND state = ?`)

		rows, err = db.db.QueryContext(ctx, query, pgutil.UUIDArray(projectIDs), start, end, invoiceProjectRecordStateUnapplied)

	case dbutil.Spanner:
		pids := make([][]byte, len(projectIDs))
		for i, v := range projectIDs {
			pids[i] = v.Bytes()
		}
		query = `SELECT
				id, project_id, storage, egress, segments, period_start, period_end, state
			FROM
				stripecoinpayments_invoice_project_records
			WHERE
				project_id IN UNNEST(?)
				AND period_start = ? AND period_end = ? AND state = ?`

		rows, err = db.db.QueryContext(ctx, query, pids, start, end, int(invoiceProjectRecordStateUnapplied))
	default:
		return nil, Error.New("unsupported database: %v", db.db.impl)
	}
	err = withRows(rows, err)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var record stripe.ProjectRecord
			err := rows.Scan(&record.ID, &record.ProjectID, &record.Storage, &record.Egress, &record.Segments, &record.PeriodStart, &record.PeriodEnd, &record.State)
			if err != nil {
				return Error.New("failed to scan stripe invoice project records: %w", err)
			}

			records = append(records, record)
		}
		return nil
	})
	return records, err
}

// Consume consumes invoice project record.
func (db *invoiceProjectRecords) Consume(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_StripecoinpaymentsInvoiceProjectRecord_By_Id(ctx,
		dbx.StripecoinpaymentsInvoiceProjectRecord_Id(id[:]),
		dbx.StripecoinpaymentsInvoiceProjectRecord_Update_Fields{
			State: dbx.StripecoinpaymentsInvoiceProjectRecord_State(invoiceProjectRecordStateConsumed.Int()),
		},
	)

	return err
}

// ListUnapplied returns project records page with unapplied project records.
// Cursor is not included into listing results.
func (db *invoiceProjectRecords) ListUnapplied(ctx context.Context, cursor uuid.UUID, limit int, start, end time.Time) (page stripe.ProjectRecordsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.list(ctx, cursor, limit, invoiceProjectRecordStateUnapplied.Int(), start, end)
}

func (db *invoiceProjectRecords) list(ctx context.Context, cursor uuid.UUID, limit, state int, start, end time.Time) (page stripe.ProjectRecordsPage, err error) {
	err = withRows(db.db.QueryContext(ctx, db.db.Rebind(`
		SELECT
			id, project_id, storage, egress, segments, period_start, period_end, state
		FROM
			stripecoinpayments_invoice_project_records
		WHERE
			id > ? AND period_start = ? AND period_end = ? AND state = ?
		ORDER BY id
		LIMIT ?
	`), cursor, start, end, state, limit+1))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var record stripe.ProjectRecord
			err := rows.Scan(&record.ID, &record.ProjectID, &record.Storage, &record.Egress, &record.Segments, &record.PeriodStart, &record.PeriodEnd, &record.State)
			if err != nil {
				return Error.New("failed to scan stripe invoice project records: %w", err)
			}

			page.Records = append(page.Records, record)
		}
		return nil
	})
	if err != nil {
		return stripe.ProjectRecordsPage{}, err
	}

	if len(page.Records) == limit+1 {
		page.Next = true

		page.Records = page.Records[:len(page.Records)-1]

		page.Cursor = page.Records[len(page.Records)-1].ID
	}

	return page, nil
}

// fromDBXInvoiceProjectRecord converts *dbx.StripecoinpaymentsInvoiceProjectRecord to *stripecoinpayments.ProjectRecord.
func fromDBXInvoiceProjectRecord(dbxRecord *dbx.StripecoinpaymentsInvoiceProjectRecord) (*stripe.ProjectRecord, error) {
	id, err := uuid.FromBytes(dbxRecord.Id)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	projectID, err := uuid.FromBytes(dbxRecord.ProjectId)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var segments float64
	if dbxRecord.Segments != nil {
		segments = float64(*dbxRecord.Segments)
	}

	return &stripe.ProjectRecord{
		ID:          id,
		ProjectID:   projectID,
		Storage:     dbxRecord.Storage,
		Egress:      dbxRecord.Egress,
		Segments:    segments,
		PeriodStart: dbxRecord.PeriodStart,
		PeriodEnd:   dbxRecord.PeriodEnd,
		State:       dbxRecord.State,
	}, nil
}
