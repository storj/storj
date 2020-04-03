// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// invoices is an implementation of payments.Invoices.
//
// architecture: Service
type invoices struct {
	service *Service
}

// List returns a list of invoices for a given payment account.
func (invoices *invoices) List(ctx context.Context, userID uuid.UUID) (invoicesList []payments.Invoice, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.InvoiceListParams{
		Customer: &customerID,
	}

	invoicesIterator := invoices.service.stripeClient.Invoices.List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		invoicesList = append(invoicesList, payments.Invoice{
			ID:          stripeInvoice.ID,
			Description: stripeInvoice.Description,
			Amount:      stripeInvoice.Total,
			Status:      string(stripeInvoice.Status),
			Link:        stripeInvoice.InvoicePDF,
			Start:       time.Unix(stripeInvoice.PeriodStart, 0),
		})
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return invoicesList, nil
}
