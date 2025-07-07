// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
)

var _ billing.Observer = (*InvoiceTokenPaymentObserver)(nil)

// InvoiceTokenPaymentObserver used to pay pending payments with STORJ tokens.
type InvoiceTokenPaymentObserver struct {
	consoleDB     DB
	invoices      payments.Invoices
	freezeService *AccountFreezeService
	nowFn         func() time.Time
}

// NewInvoiceTokenPaymentObserver creates new observer instance.
func NewInvoiceTokenPaymentObserver(consoleDB DB, invoices payments.Invoices, freezeService *AccountFreezeService) *InvoiceTokenPaymentObserver {
	return &InvoiceTokenPaymentObserver{
		consoleDB:     consoleDB,
		invoices:      invoices,
		freezeService: freezeService,
		nowFn:         time.Now,
	}
}

// Process attempts to pay user's pending payments with tokens.
func (o *InvoiceTokenPaymentObserver) Process(ctx context.Context, transaction billing.Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := o.consoleDB.Users().Get(ctx, transaction.UserID)
	if err != nil {
		return err
	}

	if user.IsFree() {
		return nil
	}

	err = o.invoices.AttemptPayOverdueInvoicesWithTokens(ctx, user.ID)
	if err != nil {
		return err
	}

	freezes, err := o.freezeService.GetAll(ctx, user.ID)
	if err != nil {
		return err
	}

	if freezes.BillingFreeze == nil && freezes.BillingWarning == nil {
		return nil
	}

	invoices, err := o.invoices.List(ctx, user.ID)
	if err != nil {
		return err
	}

	for _, inv := range invoices {
		if inv.Status != payments.InvoiceStatusPaid {
			return nil
		}
	}

	if freezes.BillingFreeze != nil {
		err = o.freezeService.BillingUnfreezeUser(ctx, user.ID)
		if err != nil {
			return err
		}
	} else if freezes.BillingWarning != nil {
		err = o.freezeService.BillingUnWarnUser(ctx, user.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// TestSetNow allows tests to have the observer act as if the current time is whatever they want.
func (o *InvoiceTokenPaymentObserver) TestSetNow(nowFn func() time.Time) {
	o.nowFn = nowFn
}
