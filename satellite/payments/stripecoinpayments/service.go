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

// Service is an implementation for PaymentsService via Stripe and Coinpayments
type Service struct {
	customers Customers
}

// NewService creates a Service instance.
func NewService(customers Customers) *Service {
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
