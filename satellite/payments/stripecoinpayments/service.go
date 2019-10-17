// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var mon = monkit.Package()

// Error defines stripecoinpayments service error.
var Error = errs.Class("stripecoinpayments service error")

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey        string
	CoinpaymentsPublicKey  string
	CoinpaymentsPrivateKey string
}

// Service is an implementation for payment service via Stripe and Coinpayments.
type Service struct {
	customers      CustomersDB
	transactionsDB TransactionsDB
	stripeClient   *client.API
	coinpayments   coinpayments.Client
}

// NewService creates a Service instance.
func NewService(config Config, customers CustomersDB, transactionsDB TransactionsDB) *Service {
	stripeClient := client.New(config.StripeSecretKey, nil)

	coinpaymentsClient := coinpayments.NewClient(
		coinpayments.Credentials{
			PublicKey:  config.CoinpaymentsPublicKey,
			PrivateKey: config.CoinpaymentsPrivateKey,
		},
	)

	return &Service{
		customers:      customers,
		transactionsDB: transactionsDB,
		stripeClient:   stripeClient,
		coinpayments:   *coinpaymentsClient,
	}
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts(userID uuid.UUID) payments.Accounts {
	return &accounts{service: service}
}
