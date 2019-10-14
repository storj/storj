// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var mon = monkit.Package()

// ErrorStripe is stripe error type
var ErrorStripe = errs.Class("stripe API error")

// Service is an implementation for payment service via Stripe and Coinpayments
type Service struct {
	customers CustomersDB
}

// NewService creates a Service instance.
func NewService(customers CustomersDB) *Service {
	return &Service{
		customers: customers,
	}
}

// Setup creates a payment account for the user.
func (service *Service) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	if _, err := customer.New(params); err != nil {
		return ErrorStripe.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return service.customers.Insert(ctx, userID, email)
}

// Balance returns an integer amount in cents that represents the current balance of payment account.
func (service *Service) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := service.customers.GetCustomerID(ctx, userID)
	if err != nil {
		return 0, err
	}

	c, err := customer.Get(customerID, nil)
	if err != nil {
		return 0, ErrorStripe.Wrap(err)
	}

	return c.Balance, nil
}
