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

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		total := stripeInvoice.Total
		for _, line := range stripeInvoice.Lines.Data {
			// If amount is negative, this is a coupon or a credit line item.
			// Add them to the total.
			if line.Amount < 0 {
				total -= line.Amount
			}
		}

		invoicesList = append(invoicesList, payments.Invoice{
			ID:          stripeInvoice.ID,
			Description: stripeInvoice.Description,
			Amount:      total,
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

// CheckPendingItems returns if pending invoice items for a given payment account exist.
func (invoices *invoices) CheckPendingItems(ctx context.Context, userID uuid.UUID) (existingItems bool, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return false, Error.Wrap(err)
	}

	params := &stripe.InvoiceItemListParams{
		Customer: &customerID,
		Pending:  stripe.Bool(true),
	}

	itemIterator := invoices.service.stripeClient.InvoiceItems().List(params)
	for itemIterator.Next() {
		item := itemIterator.InvoiceItem()
		if item != nil {
			return true, nil
		}
	}

	if err = itemIterator.Err(); err != nil {
		return false, Error.Wrap(err)
	}

	return false, nil
}
