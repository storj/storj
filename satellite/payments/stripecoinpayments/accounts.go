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
}

// CreditCards exposes all needed functionality to manage account credit cards.
func (accounts *accounts) CreditCards() payments.CreditCards {
	return &creditCards{service: accounts.service}
}

// Setup creates a payment account for the user.
// If account is already set up it will return nil.
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	_, err = accounts.service.customers.GetCustomerID(ctx, userID)
	if err == nil {
		return nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	if _, err := accounts.service.stripeClient.Customers.New(params); err != nil {
		return Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return Error.Wrap(accounts.service.customers.Insert(ctx, userID, email))
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.customers.GetCustomerID(ctx, userID)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	c, err := accounts.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return c.Balance, nil
}

// StorjTokens exposes all storj token related functionality.
func (accounts *accounts) StorjTokens() payments.StorjTokens {
	return &storjTokens{service: accounts.service}
}
