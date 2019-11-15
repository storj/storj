// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mockpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
)

var (
	// Error defines mock payment service error.
	Error = errs.Class("mock payment service error")

	mon = monkit.Package()
)

var _ payments.Accounts = (*accounts)(nil)

// accounts is a mock implementation of payments.Accounts.
type accounts struct{}

var _ payments.CreditCards = (*creditCards)(nil)

// creditCards is a mock implementation of payments.CreditCards.
type creditCards struct{}

var _ payments.Invoices = (*invoices)(nil)

// invoices is a mock implementation of payments.Invoices.
type invoices struct{}

var _ payments.StorjTokens = (*storjTokens)(nil)

// storjTokens is a mock implementation of payments.StorjTokens.
type storjTokens struct{}

// Accounts exposes all needed functionality to manage payment accounts.
func Accounts() payments.Accounts {
	return &accounts{}
}

// CreditCards exposes all needed functionality to manage account credit cards.
func (accounts *accounts) CreditCards() payments.CreditCards {
	return &creditCards{}
}

// Invoices exposes all needed functionality to manage account invoices.
func (accounts *accounts) Invoices() payments.Invoices {
	return &invoices{}
}

// StorjTokens exposes all storj token related functionality.
func (accounts *accounts) StorjTokens() payments.StorjTokens {
	return &storjTokens{}
}

// Setup creates a payment account for the user.
// If account is already set up it will return nil.
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	return nil
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return 0, nil
}

// ProjectCharges returns how much money current user will be charged for each project.
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID) (charges []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return []payments.ProjectCharge{}, nil
}

// List returns a list of credit cards for a given payment account.
func (creditCards *creditCards) List(ctx context.Context, userID uuid.UUID) (_ []payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return []payments.CreditCard{}, nil
}

// Add is used to save new credit card, attach it to payment account and make it default.
func (creditCards *creditCards) Add(ctx context.Context, userID uuid.UUID, cardToken string) (err error) {
	defer mon.Task()(&ctx, userID, cardToken)(&err)

	return nil
}

// MakeDefault makes a credit card default payment method.
func (creditCards *creditCards) MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, userID, cardID)(&err)

	return nil
}

// Remove is used to remove credit card from payment account.
func (creditCards *creditCards) Remove(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	return nil
}

// List returns a list of invoices for a given payment account.
func (invoices *invoices) List(ctx context.Context, userID uuid.UUID) (_ []payments.Invoice, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	return []payments.Invoice{}, nil
}

// Deposit creates new deposit transaction.
func (tokens *storjTokens) Deposit(ctx context.Context, userID uuid.UUID, amount *payments.TokenAmount) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx, userID, amount)(&err)

	return nil, Error.Wrap(errs.New("can not make deposit"))
}

// ListTransactionInfos returns empty transaction infos slice.
func (tokens *storjTokens) ListTransactionInfos(ctx context.Context, userID uuid.UUID) (_ []payments.TransactionInfo, err error) {
	defer mon.Task()(&ctx, userID)(&err)
	return ([]payments.TransactionInfo)(nil), nil
}
