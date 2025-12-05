// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"

	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments"
)

type paymentIntents struct {
	service *Service
}

// ChargeCard attempts to charge a credit card.
func (pi *paymentIntents) ChargeCard(ctx context.Context, request payments.ChargeCardRequest) (*payments.ChargeCardResponse, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	customerID, err := pi.service.db.Customers().GetCustomerID(ctx, request.UserID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	intent, err := pi.service.stripeClient.PaymentIntents().New(&stripe.PaymentIntentParams{
		Params:             stripe.Params{Context: ctx},
		Amount:             stripe.Int64(request.Amount),
		Customer:           stripe.String(customerID),
		PaymentMethod:      stripe.String(request.CardID),
		Currency:           stripe.String(string(stripe.CurrencyUSD)),
		ConfirmationMethod: stripe.String(string(stripe.PaymentIntentConfirmationMethodAutomatic)),
		Metadata:           request.Metadata,
	})
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return nil, Error.Wrap(err)
	}

	if intent.Status == stripe.PaymentIntentStatusRequiresConfirmation || intent.Status == stripe.PaymentIntentStatusRequiresAction {
		return &payments.ChargeCardResponse{
			Success:         false,
			ClientSecret:    intent.ClientSecret,
			PaymentIntentID: intent.ID,
		}, nil
	}

	if intent.Status != stripe.PaymentIntentStatusSucceeded {
		return nil, Error.New("Payment was not successful.")
	}

	return &payments.ChargeCardResponse{Success: true}, nil
}

// Create creates a new abstract payment intent.
func (pi *paymentIntents) Create(ctx context.Context, request payments.CreateIntentParams) (string, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	customerID, err := pi.service.db.Customers().GetCustomerID(ctx, request.UserID)
	if err != nil {
		return "", Error.Wrap(err)
	}

	params := &stripe.PaymentIntentParams{
		Params:   stripe.Params{Context: ctx},
		Amount:   stripe.Int64(request.Amount),
		Customer: stripe.String(customerID),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Metadata: request.Metadata,
	}
	if request.WithCustomCard {
		params.PaymentMethodTypes = []*string{stripe.String("card")}
	} else {
		params.AutomaticPaymentMethods = &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)}
	}

	intent, err := pi.service.stripeClient.PaymentIntents().New(params)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return "", Error.Wrap(err)
	}

	return intent.ClientSecret, nil
}
