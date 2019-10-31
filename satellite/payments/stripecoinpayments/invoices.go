// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"

	"storj.io/storj/satellite/payments"
)

// invoices is an implementation of payments.Invoices.
type invoices struct {
	service *Service
}

// List returns a list of invoices for a given payment account.
func (invoices *invoices) List(ctx context.Context, userID uuid.UUID) (invoicesList []payments.Invoice, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.customers.GetCustomerID(ctx, userID)
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
			Amount:      stripeInvoice.AmountDue,
			Status:      string(stripeInvoice.Status),
			Link:        stripeInvoice.InvoicePDF,
			End:         time.Unix(stripeInvoice.PeriodEnd, 0),
			Start:       time.Unix(stripeInvoice.PeriodStart, 0),
		})
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return invoicesList, nil
}
