// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"

	"storj.io/storj/satellite/payments"
)

// creditCards is an implementation of payments.CreditCards.
type creditCards struct {
	service *Service
}

// List returns a list of PaymentMethods for a given Customer.
func (creditCards *creditCards) List(ctx context.Context, userID uuid.UUID) (cards []payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := creditCards.service.customers.GetCustomerID(ctx, userID)
	if err != nil {
		return nil, err
	}

	params := &stripe.PaymentMethodListParams{
		Customer: &customerID,
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods.List(params)
	for paymentMethodsIterator.Next() {
		stripeCard := paymentMethodsIterator.PaymentMethod()

		cards = append(cards, payments.CreditCard{
			ExpMonth: int(stripeCard.Card.ExpMonth),
			ExpYear:  int(stripeCard.Card.ExpYear),
			Brand:    string(stripeCard.Card.Brand),
			Last4:    stripeCard.Card.Last4,
		})
	}

	if err = paymentMethodsIterator.Err(); err != nil {
		return nil, ErrorStripe.Wrap(err)
	}

	return cards, nil
}
