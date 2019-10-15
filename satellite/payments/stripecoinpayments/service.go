// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
)

var mon = monkit.Package()

// ErrorStripe is stripe error type.
var ErrorStripe = errs.Class("stripe API error")

// Config stores needed information for payment service initialization
type Config struct {
	secretKey string
}

// Service is an implementation for payment service via Stripe and Coinpayments.
type Service struct {
	customers    CustomersDB
	stripeClient *client.API
}

// NewService creates a Service instance.
func NewService(config Config, customers CustomersDB) *Service {
	stripeClient := client.New(config.secretKey, nil)

	return &Service{
		customers:    customers,
		stripeClient: stripeClient,
	}
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts(userID uuid.UUID) payments.Accounts {
	return &accounts{service: service}
}
