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
	consoleDB     DB
	invoices      payments.Invoices
	freezeService *AccountFreezeService
}

// NewInvoiceTokenPaymentObserver creates new observer instance.
func NewInvoiceTokenPaymentObserver(consoleDB DB, invoices payments.Invoices, freezeService *AccountFreezeService) *InvoiceTokenPaymentObserver {
	return &InvoiceTokenPaymentObserver{
		consoleDB:     consoleDB,
		invoices:      invoices,
		freezeService: freezeService,
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

	err = o.invoices.AttemptPayOverdueInvoicesWithTokens(ctx, user.ID)
	if err != nil {
		return err
	}

	freeze, warning, err := o.freezeService.GetAll(ctx, user.ID)
	if err != nil {
		return err
	}

	if freeze == nil && warning == nil {
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

	if freeze != nil {
		err = o.freezeService.UnfreezeUser(ctx, user.ID)
		if err != nil {
			return err
		}
	} else if warning != nil {
		err = o.freezeService.UnWarnUser(ctx, user.ID)
		if err != nil {
			return err
		}
	}

	return nil
}
