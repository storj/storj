// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"

	"github.com/stripe/stripe-go/v75"
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
		Amount:        stripe.Int64(request.Amount),
		Customer:      stripe.String(customerID),
		PaymentMethod: stripe.String(request.CardID),
		Currency:      stripe.String(string(stripe.CurrencyUSD)),
		Metadata:      request.Metadata,
	})
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			if stripeErr.PaymentIntent != nil && stripeErr.PaymentIntent.Status == stripe.PaymentIntentStatusRequiresAction {
				return &payments.ChargeCardResponse{
					Success:         false,
					ClientSecret:    stripeErr.PaymentIntent.ClientSecret,
					PaymentIntentID: stripeErr.PaymentIntent.ID,
				}, nil
			}
		}

		return nil, Error.Wrap(err)
	}

	if intent.Status != stripe.PaymentIntentStatusSucceeded {
		return nil, Error.Wrap(errs.New("Payment was not successful."))
	}

	return &payments.ChargeCardResponse{Success: true}, nil
}
