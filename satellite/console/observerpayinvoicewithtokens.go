// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
)

var _ billing.Observer = (*InvoiceTokenPaymentObserver)(nil)

// InvoiceTokenPaymentObserver used to pay pending payments with STORJ tokens.
type InvoiceTokenPaymentObserver struct {
	consoleDB DB
	payments  payments.Accounts
}

// NewInvoiceTokenPaymentObserver creates new observer instance.
func NewInvoiceTokenPaymentObserver(consoleDB DB, payments payments.Accounts) *InvoiceTokenPaymentObserver {
	return &InvoiceTokenPaymentObserver{
		consoleDB: consoleDB,
		payments:  payments,
	}
}

// Process attempts to pay user's pending payments with tokens.
func (o *InvoiceTokenPaymentObserver) Process(ctx context.Context, transaction billing.Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := o.consoleDB.Users().Get(ctx, transaction.UserID)
	if err != nil {
		return err
	}

	if !user.PaidTier {
		return nil
	}

	err = o.payments.Invoices().AttemptPayOverdueInvoicesWithTokens(ctx, user.ID)
	if err != nil {
		return err
	}

	return nil
}
