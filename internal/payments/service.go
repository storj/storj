// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Service is interfaces that defines behavior for working with payments
type Service interface {
	CreateCustomer(params CustomerParams) (*stripe.Customer, error)
}

// StripeService works with stripe network through stripe-go client
type StripeService struct {
	log    *zap.Logger
	client *client.API
}

// CustomerParams contains info neede to create new stripe customer
type CustomerParams struct {
	Email       string
	Name        string
	Description string
	SourceToken string
}

// NewService creates new instance of StripeService initialized with API key
func NewService(log *zap.Logger, apiKey string) *StripeService {
	stripe.DefaultLeveledLogger = wrapLogger(log)

	sc := new(client.API)
	sc.Init(apiKey, nil)

	return &StripeService{
		log:    log,
		client: sc,
	}
}

// CreateCustomer creates new customer from CustomerParams struct
// sets default payment to one of the predefined testing VISA credit cards
func (s *StripeService) CreateCustomer(params CustomerParams) (*stripe.Customer, error) {
	cparams := &stripe.CustomerParams{
		Email:       stripe.String(params.Email),
		Name:        stripe.String(params.Name),
		Description: stripe.String(params.Description),
	}

	// Set default source (payment instrument)
	//if params.SourceToken != "" {
	//	err := cparams.SetSource(params.SourceToken)
	//	if err != nil {
	//		return nil, errs.New("stripe error: %s", err)
	//	}
	//}

	// TODO: delete after migrating from test environment
	err := cparams.SetSource("tok_visa")
	if err != nil {
		return nil, errs.New("stripe error: %s", err)
	}

	return s.client.Customers.New(cparams)
}
