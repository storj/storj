// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"

	"storj.io/storj/satellite/payments"
)

// accounts is an implementation of payments.Accounts.
type accounts struct {
	service *Service
	userID  uuid.UUID
}

// CreditCards exposes all needed functionality to manage account credit cards.
func (accounts *accounts) CreditCards() payments.CreditCards {
	return &creditCards{service: accounts.service}
}

// Setup creates a payment account for the user.
func (accounts *accounts) Setup(ctx context.Context, email string) (err error) {
	defer mon.Task()(&ctx, accounts.userID, email)(&err)

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	if _, err := accounts.service.stripeClient.Customers.New(params); err != nil {
		return ErrorStripe.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return accounts.service.customers.Insert(ctx, accounts.userID, email)
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx, accounts.userID)(&err)

	customerID, err := accounts.service.customers.GetCustomerID(ctx, accounts.userID)
	if err != nil {
		return 0, err
	}

	c, err := accounts.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return 0, ErrorStripe.Wrap(err)
	}

	return c.Balance, nil
}
