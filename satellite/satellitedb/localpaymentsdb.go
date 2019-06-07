// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// localPaymentsDBErr is error wrapper for local payments db
var localPaymentsDBErr = errs.Class("local payments db error")

// localPaymentsDB is db implementation for local payments db
type localPaymentsDB struct {
	db *dbx.DB
}

// CreateInvoice stores new invoice into db
func (db *localPaymentsDB) CreateInvoice(ctx context.Context, invoice payments.Invoice) (*payments.Invoice, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	tx, err := db.db.Open(ctx)
	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	var dbxInvoice *dbx.LocalInvoice
	err = withTX(tx, func() (err error) {
		dbxInvoice, err = tx.Create_LocalInvoice(ctx,
			dbx.LocalInvoice_Id(id[:]),
			dbx.LocalInvoice_PaymentMethodId(invoice.PaymentMethodID),
			dbx.LocalInvoice_Amount(invoice.Amount),
			dbx.LocalInvoice_Currency(string(invoice.Currency)),
			dbx.LocalInvoice_Status(string(invoice.Status)),
		)
		if err != nil {
			return
		}

		// create line items
		for _, item := range invoice.LineItems {
			var id *uuid.UUID

			id, err = uuid.New()
			if err != nil {
				return
			}

			_, err = tx.Create_LocalInvoiceLineItem(ctx,
				dbx.LocalInvoiceLineItem_Id(id[:]),
				dbx.LocalInvoiceLineItem_InvoiceId(dbxInvoice.Id),
				dbx.LocalInvoiceLineItem_Key(item.Key),
				dbx.LocalInvoiceLineItem_Quantity(item.Quantity),
				dbx.LocalInvoiceLineItem_Amount(item.Amount),
			)
			if err != nil {
				return
			}
		}

		// create custom fields
		for _, field := range invoice.CustomFields {
			var id *uuid.UUID

			id, err = uuid.New()
			if err != nil {
				return
			}

			_, err = tx.Create_LocalInvoiceCustomField(ctx,
				dbx.LocalInvoiceCustomField_Id(id[:]),
				dbx.LocalInvoiceCustomField_InvoiceId(dbxInvoice.Id),
				dbx.LocalInvoiceCustomField_Name(field.Name),
				dbx.LocalInvoiceCustomField_Value(field.Value),
			)
			if err != nil {
				return
			}
		}

		return nil
	})

	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	invoice.ID = dbxInvoice.Id
	invoice.CreatedAt = dbxInvoice.CreatedAt
	return &invoice, nil
}

// GetInvoice retrieve invoice from db by it's id
func (db *localPaymentsDB) GetInvoice(ctx context.Context, id []byte) (*payments.Invoice, error) {
	dbxInvoice, err := db.db.Get_LocalInvoice_By_Id(ctx, dbx.LocalInvoice_Id(id))
	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	// get line items
	dbxItems, err := db.db.All_LocalInvoiceLineItem_By_InvoiceId(ctx, dbx.LocalInvoiceLineItem_InvoiceId(id))
	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	// get custom fields
	dbxFields, err := db.db.All_LocalInvoiceCustomField_By_InvoiceId(ctx, dbx.LocalInvoiceCustomField_InvoiceId(id))
	if err != nil {
		return nil, localPaymentsDBErr.Wrap(err)
	}

	var lineItems []payments.LineItem
	for _, dbxItem := range dbxItems {
		lineItems = append(lineItems,
			payments.LineItem{
				Key:      dbxItem.Key,
				Quantity: dbxItem.Quantity,
				Amount:   dbxItem.Amount,
			},
		)
	}

	var customFields []payments.CustomField
	for _, dbxField := range dbxFields {
		customFields = append(customFields,
			payments.CustomField{
				Name:  dbxField.Name,
				Value: dbxField.Value,
			},
		)
	}

	return &payments.Invoice{
		ID:              dbxInvoice.Id,
		PaymentMethodID: dbxInvoice.PaymentMethodId,
		Amount:          dbxInvoice.Amount,
		Currency:        payments.Currency(dbxInvoice.Currency),
		Status:          payments.InvoiceStatus(dbxInvoice.Status),
		LineItems:       lineItems,
		CustomFields:    customFields,
		CreatedAt:       dbxInvoice.CreatedAt,
	}, nil
}

// withTX is a helper method for transaction scoped execution.
// Consumes provided tx, so it can no longer be used
func withTX(tx *dbx.Tx, cb func() error) (err error) {
	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	return cb()
}
