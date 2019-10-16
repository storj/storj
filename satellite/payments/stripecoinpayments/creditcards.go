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
	userID  uuid.UUID
}

// List returns a list of PaymentMethods for a given Customer.
func (creditCards *creditCards) List(ctx context.Context) (cards []payments.CreditCard, err error) {
	defer mon.Task()(&ctx, creditCards.userID)(&err)

	customerID, err := creditCards.service.customers.GetCustomerID(ctx, creditCards.userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.PaymentMethodListParams{
		Customer: &customerID,
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods.List(params)
	for paymentMethodsIterator.Next() {
		stripeCard := paymentMethodsIterator.PaymentMethod()

		cards = append(cards, payments.CreditCard{
			ID:       []byte(stripeCard.ID),
			ExpMonth: int(stripeCard.Card.ExpMonth),
			ExpYear:  int(stripeCard.Card.ExpYear),
			Brand:    string(stripeCard.Card.Brand),
			Last4:    stripeCard.Card.Last4,
		})
	}

	if err = paymentMethodsIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return cards, nil
}

// Add is used to save new credit card and attach it to payment account.
func (creditCards *creditCards) Add(ctx context.Context, cardToken string) (err error) {
	defer mon.Task()(&ctx, creditCards.userID, cardToken)(&err)

	customerID, err := creditCards.service.customers.GetCustomerID(ctx, creditCards.userID)
	if err != nil {
		return err
	}

	cardParams := &stripe.PaymentMethodParams{
		Type: stripe.String(string(stripe.PaymentMethodTypeCard)),
		Card: &stripe.PaymentMethodCardParams{Token: &cardToken},
	}

	card, err := creditCards.service.stripeClient.PaymentMethods.New(cardParams)
	if err != nil {
		return Error.Wrap(err)
	}

	attachParams := &stripe.PaymentMethodAttachParams{
		Customer: &customerID,
	}

	_, err = creditCards.service.stripeClient.PaymentMethods.Attach(card.ID, attachParams)
	if err != nil {
		// TODO: handle created but not attached card manually?
		return Error.Wrap(err)
	}

	return nil
}
